import type { ReactNode } from 'react'

import { useOrderStore } from './store/useOrderStore'
import type { Order, OrderStatus } from './types'

const statusTone: Record<
  OrderStatus,
  { pill: string; dot: string; label: string }
> = {
  Paid: {
    pill:
      'border-[rgba(189,228,197,0.28)] bg-[rgba(189,228,197,0.10)] text-[#d8e6d7]',
    dot: 'bg-[#bde4c5]',
    label: 'Payment captured',
  },
  Pending: {
    pill:
      'border-[rgba(245,213,164,0.26)] bg-[rgba(245,213,164,0.10)] text-[#f5d5a4]',
    dot: 'bg-[#f5d5a4]',
    label: 'Awaiting reconciliation',
  },
  Failed: {
    pill:
      'border-[rgba(232,164,164,0.28)] bg-[rgba(232,164,164,0.10)] text-[#efc2c2]',
    dot: 'bg-[#ef9c9c]',
    label: 'Payment declined',
  },
  Cancelled: {
    pill:
      'border-[rgba(178,177,173,0.24)] bg-[rgba(178,177,173,0.10)] text-[#d0cdca]',
    dot: 'bg-[#b2b1ad]',
    label: 'Cancelled before payment',
  },
}

function formatAmount(amount: number) {
  return new Intl.NumberFormat('en-US').format(amount)
}

function toneClasses(tone: 'neutral' | 'success' | 'danger') {
  if (tone === 'success') {
    return 'border-[rgba(189,228,197,0.24)] bg-[rgba(189,228,197,0.08)] text-[#f0f4ec]'
  }

  if (tone === 'danger') {
    return 'border-[rgba(232,164,164,0.24)] bg-[rgba(232,164,164,0.08)] text-[#f5d9d5]'
  }

  return 'border-[rgba(226,226,226,0.2)] bg-[rgba(255,255,255,0.05)] text-[#faf9f6]'
}

function SectionShell({
  eyebrow,
  title,
  description,
  children,
}: {
  eyebrow: string
  title: string
  description: string
  children: ReactNode
}) {
  return (
    <section className="rounded-[28px] border border-[rgba(226,226,226,0.18)] bg-[rgba(255,255,255,0.04)] p-6 shadow-[0_18px_40px_rgba(0,0,0,0.18)] backdrop-blur-[2px] sm:p-8">
      <p className="text-[11px] uppercase tracking-[0.32em] text-[#868584]">
        {eyebrow}
      </p>
      <div className="mt-4 space-y-2">
        <h2 className="text-3xl leading-[1.08] tracking-[-0.04em] text-[#faf9f6] sm:text-[2.5rem]">
          {title}
        </h2>
        <p className="max-w-2xl text-[15px] leading-7 text-[#afaeac] sm:text-base">
          {description}
        </p>
      </div>
      <div className="mt-8">{children}</div>
    </section>
  )
}

function Field({
  label,
  hint,
  value,
  onChange,
  placeholder,
  type = 'text',
}: {
  label: string
  hint?: string
  value: string
  onChange: (value: string) => void
  placeholder: string
  type?: 'text' | 'number'
}) {
  return (
    <label className="block space-y-3">
      <div className="flex items-end justify-between gap-4">
        <span className="text-sm text-[#faf9f6]">{label}</span>
        {hint ? (
          <span className="text-xs uppercase tracking-[0.24em] text-[#666469]">
            {hint}
          </span>
        ) : null}
      </div>
      <input
        className="w-full rounded-[20px] border border-[rgba(226,226,226,0.18)] bg-[rgba(255,255,255,0.03)] px-4 py-3 text-base text-[#faf9f6] outline-none transition placeholder:text-[#666469] focus:border-[rgba(250,249,246,0.35)]"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        type={type}
      />
    </label>
  )
}

function StatusBadge({ status }: { status: OrderStatus }) {
  const tone = statusTone[status]

  return (
    <span
      className={`inline-flex items-center gap-2 rounded-full border px-3 py-1 text-xs uppercase tracking-[0.24em] ${tone.pill}`}
    >
      <span className={`h-2 w-2 rounded-full ${tone.dot}`}></span>
      {status}
    </span>
  )
}

function OrderCard({ order }: { order: Order }) {
  const tone = statusTone[order.status]

  return (
    <div className="rounded-[24px] border border-[rgba(226,226,226,0.16)] bg-[rgba(10,9,7,0.42)] p-5">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <p className="text-[11px] uppercase tracking-[0.32em] text-[#868584]">
            Order snapshot
          </p>
          <h3 className="mt-3 text-2xl tracking-[-0.04em] text-[#faf9f6]">
            {order.item_name}
          </h3>
        </div>
        <StatusBadge status={order.status} />
      </div>

      <dl className="mt-6 grid gap-4 text-sm text-[#afaeac] sm:grid-cols-2">
        <div>
          <dt className="text-[11px] uppercase tracking-[0.24em] text-[#666469]">
            Order ID
          </dt>
          <dd className="mt-2 break-all font-mono text-[13px] text-[#faf9f6]">
            {order.id}
          </dd>
        </div>
        <div>
          <dt className="text-[11px] uppercase tracking-[0.24em] text-[#666469]">
            Customer
          </dt>
          <dd className="mt-2 text-[#faf9f6]">{order.customer_id}</dd>
        </div>
        <div>
          <dt className="text-[11px] uppercase tracking-[0.24em] text-[#666469]">
            Amount
          </dt>
          <dd className="mt-2 text-[#faf9f6]">{formatAmount(order.amount)}</dd>
        </div>
        <div>
          <dt className="text-[11px] uppercase tracking-[0.24em] text-[#666469]">
            Payment
          </dt>
          <dd className="mt-2 text-[#faf9f6]">
            {order.payment_transaction_id ?? 'Not available'}
          </dd>
        </div>
      </dl>

      <div className="mt-6 rounded-[20px] border border-[rgba(226,226,226,0.12)] bg-[rgba(255,255,255,0.03)] px-4 py-3 text-sm text-[#afaeac]">
        {tone.label}
      </div>
    </div>
  )
}

function App() {
  const {
    banner,
    createForm,
    createResult,
    dismissBanner,
    loadOrder,
    loadRecentOrder,
    loadRevenue,
    lookupOrderId,
    recentOrders,
    resetIdempotencyKey,
    revenueCustomerId,
    revenueResult,
    selectedOrder,
    setCreateField,
    setLookupOrderId,
    setRevenueCustomerId,
    submitOrder,
    cancelSelectedOrder,
  } = useOrderStore()

  return (
    <main className="mx-auto flex min-h-screen w-full max-w-[1500px] flex-col gap-6 px-4 py-6 sm:px-6 lg:px-8">
      <section className="overflow-hidden rounded-[32px] border border-[rgba(226,226,226,0.18)] bg-[linear-gradient(160deg,rgba(255,255,255,0.06),rgba(255,255,255,0.02))] p-6 shadow-[0_24px_60px_rgba(0,0,0,0.24)] sm:p-8 lg:p-10">
        <div className="grid gap-10 lg:grid-cols-[1.2fr_0.8fr]">
          <div className="space-y-6">
            <p className="text-[11px] uppercase tracking-[0.34em] text-[#868584]">
              Simple order workflow
            </p>
            <div className="space-y-4">
              <h1 className="max-w-3xl text-5xl leading-[0.94] tracking-[-0.06em] text-[#faf9f6] sm:text-6xl lg:text-[4.8rem]">
                Warm, minimal order operations for a two-service backend.
              </h1>
              <p className="max-w-2xl text-base leading-7 text-[#afaeac] sm:text-lg">
                Create orders with an idempotency key, inspect payment outcomes,
                cancel pending records, and pull paid-order revenue without
                leaving the page.
              </p>
            </div>

            <div className="grid gap-4 sm:grid-cols-3">
              <div className="rounded-[24px] border border-[rgba(226,226,226,0.14)] bg-[rgba(255,255,255,0.04)] p-4">
                <p className="text-[11px] uppercase tracking-[0.28em] text-[#666469]">
                  Frontend stack
                </p>
                <p className="mt-3 text-xl tracking-[-0.04em] text-[#faf9f6]">
                  React + Zustand
                </p>
              </div>
              <div className="rounded-[24px] border border-[rgba(226,226,226,0.14)] bg-[rgba(255,255,255,0.04)] p-4">
                <p className="text-[11px] uppercase tracking-[0.28em] text-[#666469]">
                  API path
                </p>
                <p className="mt-3 text-xl tracking-[-0.04em] text-[#faf9f6]">
                  <code className="font-mono text-[0.95rem]">/api/orders</code>
                </p>
              </div>
              <div className="rounded-[24px] border border-[rgba(226,226,226,0.14)] bg-[rgba(255,255,255,0.04)] p-4">
                <p className="text-[11px] uppercase tracking-[0.28em] text-[#666469]">
                  Recent orders
                </p>
                <p className="mt-3 text-xl tracking-[-0.04em] text-[#faf9f6]">
                  {recentOrders.length}
                </p>
              </div>
            </div>
          </div>

          <div className="relative overflow-hidden rounded-[28px] border border-[rgba(226,226,226,0.16)] bg-[rgba(255,255,255,0.04)] p-6">
            <div className="absolute inset-x-6 top-6 h-32 rounded-full bg-[radial-gradient(circle,rgba(245,213,164,0.18),rgba(245,213,164,0))] blur-2xl"></div>
            <div className="relative space-y-5">
              <div>
                <p className="text-[11px] uppercase tracking-[0.28em] text-[#868584]">
                  Operational notes
                </p>
                <h2 className="mt-3 text-3xl leading-[1.04] tracking-[-0.04em] text-[#faf9f6]">
                  Designed around the backend’s actual behavior.
                </h2>
              </div>
              <ul className="space-y-3 text-sm leading-7 text-[#afaeac]">
                <li>
                  Create requests require an <code>Idempotency-Key</code> header.
                </li>
                <li>
                  Revenue includes only <code>Paid</code> orders.
                </li>
                <li>
                  Payment timeouts keep the order in <code>Pending</code>.
                </li>
                <li>
                  Only <code>Pending</code> orders can be cancelled.
                </li>
              </ul>
            </div>
          </div>
        </div>
      </section>

      {banner ? (
        <div
          className={`flex flex-col gap-3 rounded-[22px] border px-5 py-4 sm:flex-row sm:items-center sm:justify-between ${toneClasses(banner.tone)}`}
        >
          <p className="text-sm leading-6">{banner.message}</p>
          <button
            className="inline-flex w-fit rounded-full border border-[rgba(250,249,246,0.18)] px-4 py-2 text-xs uppercase tracking-[0.24em] text-[#faf9f6] transition hover:border-[rgba(250,249,246,0.35)]"
            onClick={dismissBanner}
          >
            Dismiss
          </button>
        </div>
      ) : null}

      <div className="grid gap-6 xl:grid-cols-[1.1fr_0.9fr]">
        <SectionShell
          eyebrow="Create"
          title="Issue an order request"
          description="The form sends the exact backend payload and surfaces the idempotency contract instead of hiding it."
        >
          <form
            className="grid gap-4"
            onSubmit={(event) => {
              event.preventDefault()
              void submitOrder()
            }}
          >
            <div className="grid gap-4 md:grid-cols-2">
              <Field
                label="Customer ID"
                value={createForm.customerId}
                onChange={(value) => setCreateField('customerId', value)}
                placeholder="cust-42"
              />
              <Field
                label="Item name"
                value={createForm.itemName}
                onChange={(value) => setCreateField('itemName', value)}
                placeholder="forest notebook"
              />
            </div>

            <div className="grid gap-4 md:grid-cols-[0.55fr_1fr]">
              <Field
                label="Amount"
                hint="Integer only"
                value={createForm.amount}
                onChange={(value) => setCreateField('amount', value)}
                placeholder="500"
                type="number"
              />
              <div className="space-y-3">
                <div className="flex items-end justify-between gap-4">
                  <label className="text-sm text-[#faf9f6]">Idempotency key</label>
                  <button
                    className="text-xs uppercase tracking-[0.24em] text-[#868584] transition hover:text-[#faf9f6]"
                    type="button"
                    onClick={resetIdempotencyKey}
                  >
                    Regenerate
                  </button>
                </div>
                <input
                  className="w-full rounded-[20px] border border-[rgba(226,226,226,0.18)] bg-[rgba(255,255,255,0.03)] px-4 py-3 text-base text-[#faf9f6] outline-none transition placeholder:text-[#666469] focus:border-[rgba(250,249,246,0.35)]"
                  value={createForm.idempotencyKey}
                  onChange={(event) =>
                    setCreateField('idempotencyKey', event.target.value)
                  }
                  placeholder="order-7f3a2d91"
                />
              </div>
            </div>

            <div className="flex flex-wrap items-center gap-3 pt-2">
              <button
                className="inline-flex rounded-full bg-[#353534] px-5 py-3 text-sm text-[#faf9f6] transition hover:bg-[#454441] disabled:cursor-not-allowed disabled:opacity-60"
                disabled={createResult.loading}
                type="submit"
              >
                {createResult.loading ? 'Submitting...' : 'Create order'}
              </button>
              <p className="text-sm text-[#868584]">
                Example payload: <code>cust-1</code>, <code>book</code>,{' '}
                <code>500</code>
              </p>
            </div>
          </form>

          {createResult.error ? (
            <p className="mt-5 text-sm text-[#efc2c2]">{createResult.error}</p>
          ) : null}

          {createResult.data ? (
            <div className="mt-6">
              <OrderCard order={createResult.data} />
            </div>
          ) : null}
        </SectionShell>

        <SectionShell
          eyebrow="Lookup"
          title="Inspect or cancel a stored order"
          description="Fetch any order by ID, then cancel it only when the backend still considers it pending."
        >
          <div className="grid gap-4">
            <form
              className="grid gap-4 sm:grid-cols-[1fr_auto]"
              onSubmit={(event) => {
                event.preventDefault()
                void loadOrder()
              }}
            >
              <Field
                label="Order ID"
                value={lookupOrderId}
                onChange={setLookupOrderId}
                placeholder="Paste an order UUID"
              />
              <button
                className="mt-[2.15rem] inline-flex h-fit rounded-full bg-[#353534] px-5 py-3 text-sm text-[#faf9f6] transition hover:bg-[#454441] disabled:cursor-not-allowed disabled:opacity-60"
                disabled={selectedOrder.loading}
                type="submit"
              >
                {selectedOrder.loading ? 'Loading...' : 'Fetch order'}
              </button>
            </form>

            {recentOrders.length ? (
              <div className="rounded-[24px] border border-[rgba(226,226,226,0.12)] bg-[rgba(255,255,255,0.03)] p-4">
                <p className="text-[11px] uppercase tracking-[0.28em] text-[#868584]">
                  Recent activity
                </p>
                <div className="mt-4 flex flex-wrap gap-3">
                  {recentOrders.map((order) => (
                    <button
                      key={order.id}
                      className="rounded-full border border-[rgba(226,226,226,0.16)] px-4 py-2 text-left text-sm text-[#faf9f6] transition hover:border-[rgba(250,249,246,0.32)]"
                      onClick={() => loadRecentOrder(order)}
                      type="button"
                    >
                      {order.customer_id} · {order.item_name}
                    </button>
                  ))}
                </div>
              </div>
            ) : null}

            {selectedOrder.error ? (
              <p className="text-sm text-[#efc2c2]">{selectedOrder.error}</p>
            ) : null}

            {selectedOrder.data ? (
              <div className="space-y-4">
                <OrderCard order={selectedOrder.data} />
                <div className="flex flex-wrap gap-3">
                  <button
                    className="inline-flex rounded-full border border-[rgba(226,226,226,0.18)] px-5 py-3 text-sm text-[#faf9f6] transition hover:border-[rgba(250,249,246,0.35)] disabled:cursor-not-allowed disabled:opacity-50"
                    disabled={
                      selectedOrder.loading || selectedOrder.data.status !== 'Pending'
                    }
                    onClick={() => void cancelSelectedOrder()}
                    type="button"
                  >
                    Cancel pending order
                  </button>
                  <button
                    className="inline-flex rounded-full border border-[rgba(226,226,226,0.18)] px-5 py-3 text-sm text-[#faf9f6] transition hover:border-[rgba(250,249,246,0.35)]"
                    onClick={() => void loadRevenue(selectedOrder.data?.customer_id)}
                    type="button"
                  >
                    Load customer revenue
                  </button>
                </div>
              </div>
            ) : null}
          </div>
        </SectionShell>
      </div>

      <SectionShell
        eyebrow="Revenue"
        title="Pull realized revenue by customer"
        description="The backend counts only paid orders, so pending, failed, and cancelled records stay out of this view."
      >
        <div className="grid gap-6 lg:grid-cols-[0.7fr_1.3fr]">
          <form
            className="space-y-4"
            onSubmit={(event) => {
              event.preventDefault()
              void loadRevenue()
            }}
          >
            <Field
              label="Customer ID"
              value={revenueCustomerId}
              onChange={setRevenueCustomerId}
              placeholder="cust-42"
            />
            <button
              className="inline-flex rounded-full bg-[#353534] px-5 py-3 text-sm text-[#faf9f6] transition hover:bg-[#454441] disabled:cursor-not-allowed disabled:opacity-60"
              disabled={revenueResult.loading}
              type="submit"
            >
              {revenueResult.loading ? 'Loading...' : 'Fetch revenue'}
            </button>
            {revenueResult.error ? (
              <p className="text-sm text-[#efc2c2]">{revenueResult.error}</p>
            ) : null}
          </form>

          <div className="grid gap-4 sm:grid-cols-3">
            <div className="rounded-[24px] border border-[rgba(226,226,226,0.14)] bg-[rgba(255,255,255,0.03)] p-5">
              <p className="text-[11px] uppercase tracking-[0.28em] text-[#666469]">
                Customer
              </p>
              <p className="mt-3 text-2xl tracking-[-0.04em] text-[#faf9f6]">
                {revenueResult.data?.customer_id ?? 'No selection'}
              </p>
            </div>
            <div className="rounded-[24px] border border-[rgba(226,226,226,0.14)] bg-[rgba(255,255,255,0.03)] p-5">
              <p className="text-[11px] uppercase tracking-[0.28em] text-[#666469]">
                Paid amount
              </p>
              <p className="mt-3 text-2xl tracking-[-0.04em] text-[#faf9f6]">
                {revenueResult.data
                  ? formatAmount(revenueResult.data.total_amount)
                  : '0'}
              </p>
            </div>
            <div className="rounded-[24px] border border-[rgba(226,226,226,0.14)] bg-[rgba(255,255,255,0.03)] p-5">
              <p className="text-[11px] uppercase tracking-[0.28em] text-[#666469]">
                Paid orders
              </p>
              <p className="mt-3 text-2xl tracking-[-0.04em] text-[#faf9f6]">
                {revenueResult.data?.orders_count ?? '0'}
              </p>
            </div>
          </div>
        </div>
      </SectionShell>
    </main>
  )
}

export default App
