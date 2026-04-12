export type OrderStatus = 'Pending' | 'Paid' | 'Failed' | 'Cancelled'

export interface Order {
  id: string
  customer_id: string
  item_name: string
  amount: number
  status: OrderStatus
  payment_transaction_id?: string
}

export interface Revenue {
  customer_id: string
  total_amount: number
  orders_count: number
}

export interface CreateOrderPayload {
  customer_id: string
  item_name: string
  amount: number
}

export interface CreateOrderForm {
  customerId: string
  itemName: string
  amount: string
  idempotencyKey: string
}

export interface Banner {
  tone: 'neutral' | 'success' | 'danger'
  message: string
}
