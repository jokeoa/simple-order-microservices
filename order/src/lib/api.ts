import type { CreateOrderPayload, Order, Revenue } from '../types'

const apiBaseUrl = (import.meta.env.VITE_API_BASE_URL ?? '/api').replace(/\/$/, '')

type JsonRecord = Record<string, unknown>

function isJsonRecord(value: unknown): value is JsonRecord {
  return typeof value === 'object' && value !== null
}

function toErrorMessage(body: unknown, fallback: string) {
  if (isJsonRecord(body) && typeof body.error === 'string') {
    return body.error
  }

  return fallback
}

async function readBody(response: Response) {
  const text = await response.text()
  if (!text) {
    return null
  }

  try {
    return JSON.parse(text) as unknown
  } catch {
    return text
  }
}

export class ApiError extends Error {
  status: number
  payload: unknown

  constructor(message: string, status: number, payload: unknown) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.payload = payload
  }
}

async function request<T>(path: string, init?: RequestInit) {
  const response = await fetch(`${apiBaseUrl}${path}`, {
    ...init,
    headers: {
      Accept: 'application/json',
      ...init?.headers,
    },
  })

  const payload = await readBody(response)

  if (!response.ok) {
    throw new ApiError(
      toErrorMessage(payload, `Request failed with status ${response.status}`),
      response.status,
      payload,
    )
  }

  return {
    data: payload as T,
    status: response.status,
  }
}

export function createOrder(
  payload: CreateOrderPayload,
  idempotencyKey: string,
) {
  return request<Order>('/orders', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Idempotency-Key': idempotencyKey,
    },
    body: JSON.stringify(payload),
  })
}

export function getOrder(orderId: string) {
  return request<Order>(`/orders/${orderId}`)
}

export function cancelOrder(orderId: string) {
  return request<Order>(`/orders/${orderId}/cancel`, {
    method: 'PATCH',
  })
}

export function getRevenue(customerId: string) {
  return request<Revenue>(
    `/orders/revenue?customer_id=${encodeURIComponent(customerId)}`,
  )
}
