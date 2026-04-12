import { create } from 'zustand'
import { createJSONStorage, persist } from 'zustand/middleware'

import { ApiError, cancelOrder, createOrder, getOrder, getRevenue } from '../lib/api'
import type { Banner, CreateOrderForm, Order, Revenue } from '../types'

const MAX_RECENT_ORDERS = 6

function nextIdempotencyKey() {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return `order-${crypto.randomUUID().slice(0, 8)}`
  }

  return `order-${Date.now().toString(36)}`
}

function mergeRecentOrders(recentOrders: Order[], nextOrder: Order) {
  return [nextOrder, ...recentOrders.filter((order) => order.id !== nextOrder.id)].slice(
    0,
    MAX_RECENT_ORDERS,
  )
}

function toMessage(error: unknown, fallback: string) {
  if (error instanceof ApiError) {
    return error.message
  }

  if (error instanceof Error && error.message) {
    return error.message
  }

  return fallback
}

function orderFromPayload(payload: unknown) {
  if (typeof payload !== 'object' || payload === null) {
    return null
  }

  if ('id' in payload && 'status' in payload) {
    return payload as Order
  }

  return null
}

interface AsyncSlice<T> {
  data: T | null
  loading: boolean
  error: string | null
}

interface OrderStore {
  createForm: CreateOrderForm
  lookupOrderId: string
  revenueCustomerId: string
  createResult: AsyncSlice<Order>
  selectedOrder: AsyncSlice<Order>
  revenueResult: AsyncSlice<Revenue>
  recentOrders: Order[]
  banner: Banner | null
  setCreateField: (field: keyof CreateOrderForm, value: string) => void
  resetIdempotencyKey: () => void
  setLookupOrderId: (value: string) => void
  setRevenueCustomerId: (value: string) => void
  dismissBanner: () => void
  submitOrder: () => Promise<void>
  loadOrder: (orderId?: string) => Promise<void>
  loadRevenue: (customerId?: string) => Promise<void>
  cancelSelectedOrder: () => Promise<void>
  loadRecentOrder: (order: Order) => void
}

export const useOrderStore = create<OrderStore>()(
  persist(
    (set, get) => ({
      createForm: {
        customerId: '',
        itemName: '',
        amount: '',
        idempotencyKey: nextIdempotencyKey(),
      },
      lookupOrderId: '',
      revenueCustomerId: '',
      createResult: { data: null, loading: false, error: null },
      selectedOrder: { data: null, loading: false, error: null },
      revenueResult: { data: null, loading: false, error: null },
      recentOrders: [],
      banner: null,
      setCreateField: (field, value) => {
        set((state) => ({
          createForm: {
            ...state.createForm,
            [field]: value,
          },
        }))
      },
      resetIdempotencyKey: () => {
        set((state) => ({
          createForm: {
            ...state.createForm,
            idempotencyKey: nextIdempotencyKey(),
          },
        }))
      },
      setLookupOrderId: (value) => {
        set({ lookupOrderId: value })
      },
      setRevenueCustomerId: (value) => {
        set({ revenueCustomerId: value })
      },
      dismissBanner: () => {
        set({ banner: null })
      },
      submitOrder: async () => {
        const { createForm } = get()
        const payload = {
          customer_id: createForm.customerId.trim(),
          item_name: createForm.itemName.trim(),
          amount: Number.parseInt(createForm.amount, 10),
        }

        set({
          createResult: { data: null, loading: true, error: null },
          banner: null,
        })

        try {
          const result = await createOrder(payload, createForm.idempotencyKey.trim())
          const message =
            result.status === 201
              ? 'Order created and payment reconciliation finished.'
              : 'Existing order replayed through the same idempotency key.'

          set((state) => ({
            createResult: { data: result.data, loading: false, error: null },
            selectedOrder: { data: result.data, loading: false, error: null },
            lookupOrderId: result.data.id,
            revenueCustomerId: result.data.customer_id,
            recentOrders: mergeRecentOrders(state.recentOrders, result.data),
            banner: { tone: 'success', message },
            createForm: {
              ...state.createForm,
              idempotencyKey: nextIdempotencyKey(),
            },
          }))
        } catch (error) {
          const pendingOrder =
            error instanceof ApiError ? orderFromPayload(error.payload) : null

          set({
            createResult: {
              data: pendingOrder,
              loading: false,
              error: toMessage(error, 'Failed to create order.'),
            },
            selectedOrder: {
              data: pendingOrder,
              loading: false,
              error: pendingOrder ? null : toMessage(error, 'Failed to load order.'),
            },
            lookupOrderId: pendingOrder?.id ?? get().lookupOrderId,
            revenueCustomerId:
              pendingOrder?.customer_id ?? get().revenueCustomerId,
            recentOrders: pendingOrder
              ? mergeRecentOrders(get().recentOrders, pendingOrder)
              : get().recentOrders,
            banner: {
              tone: 'danger',
              message:
                pendingOrder?.status === 'Pending'
                  ? 'Payment service is unavailable. The order is stored as Pending and can be retried with the same idempotency key.'
                  : toMessage(error, 'Failed to create order.'),
            },
          })
        }
      },
      loadOrder: async (orderId) => {
        const nextOrderId = (orderId ?? get().lookupOrderId).trim()
        if (!nextOrderId) {
          set({
            selectedOrder: {
              data: null,
              loading: false,
              error: 'Order ID is required.',
            },
          })
          return
        }

        set({
          selectedOrder: { data: get().selectedOrder.data, loading: true, error: null },
          lookupOrderId: nextOrderId,
          banner: null,
        })

        try {
          const result = await getOrder(nextOrderId)
          set((state) => ({
            selectedOrder: { data: result.data, loading: false, error: null },
            revenueCustomerId: result.data.customer_id,
            recentOrders: mergeRecentOrders(state.recentOrders, result.data),
          }))
        } catch (error) {
          set({
            selectedOrder: {
              data: null,
              loading: false,
              error: toMessage(error, 'Failed to load order.'),
            },
            banner: {
              tone: 'danger',
              message: toMessage(error, 'Failed to load order.'),
            },
          })
        }
      },
      loadRevenue: async (customerId) => {
        const nextCustomerId = (customerId ?? get().revenueCustomerId).trim()
        if (!nextCustomerId) {
          set({
            revenueResult: {
              data: null,
              loading: false,
              error: 'Customer ID is required.',
            },
          })
          return
        }

        set({
          revenueResult: { data: get().revenueResult.data, loading: true, error: null },
          revenueCustomerId: nextCustomerId,
          banner: null,
        })

        try {
          const result = await getRevenue(nextCustomerId)
          set({
            revenueResult: { data: result.data, loading: false, error: null },
          })
        } catch (error) {
          set({
            revenueResult: {
              data: null,
              loading: false,
              error: toMessage(error, 'Failed to load revenue.'),
            },
            banner: {
              tone: 'danger',
              message: toMessage(error, 'Failed to load revenue.'),
            },
          })
        }
      },
      cancelSelectedOrder: async () => {
        const currentOrder = get().selectedOrder.data
        if (!currentOrder) {
          return
        }

        set({
          selectedOrder: { data: currentOrder, loading: true, error: null },
          banner: null,
        })

        try {
          const result = await cancelOrder(currentOrder.id)
          set((state) => ({
            selectedOrder: { data: result.data, loading: false, error: null },
            createResult:
              state.createResult.data?.id === result.data.id
                ? { data: result.data, loading: false, error: null }
                : state.createResult,
            recentOrders: mergeRecentOrders(state.recentOrders, result.data),
            banner: {
              tone: 'success',
              message: 'Pending order cancelled.',
            },
          }))
        } catch (error) {
          set({
            selectedOrder: {
              data: currentOrder,
              loading: false,
              error: toMessage(error, 'Failed to cancel order.'),
            },
            banner: {
              tone: 'danger',
              message: toMessage(error, 'Failed to cancel order.'),
            },
          })
        }
      },
      loadRecentOrder: (order) => {
        set({
          selectedOrder: { data: order, loading: false, error: null },
          lookupOrderId: order.id,
          revenueCustomerId: order.customer_id,
          banner: null,
        })
      },
    }),
    {
      name: 'order-console',
      partialize: (state) => ({
        recentOrders: state.recentOrders,
      }),
      storage: createJSONStorage(() => localStorage),
    },
  ),
)
