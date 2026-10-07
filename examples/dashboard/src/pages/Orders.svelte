<script lang="ts">
  type OrderStatus = "Paid" | "Pending" | "Refunded";

  interface Order {
    id: string;
    customer: string;
    item: string;
    total: number;
    status: OrderStatus;
    date: string;
  }

  const orders: Order[] = [
    { id: "#1042", customer: "Northwind Labs", item: "Pro license × 5", total: 495, status: "Paid", date: "Oct 01" },
    { id: "#1041", customer: "Acme Studio", item: "Support plan (annual)", total: 240, status: "Pending", date: "Sep 30" },
    { id: "#1040", customer: "Haryanto & Co", item: "Pro license × 1", total: 99, status: "Paid", date: "Sep 29" },
    { id: "#1039", customer: "Bluebird Media", item: "Team license × 3", total: 267, status: "Refunded", date: "Sep 28" },
    { id: "#1038", customer: "Chen Consulting", item: "Support plan (monthly)", total: 24, status: "Paid", date: "Sep 27" },
    { id: "#1037", customer: "Demir Interiors", item: "Pro license × 2", total: 198, status: "Pending", date: "Sep 26" },
    { id: "#1036", customer: "Silva Ventures", item: "Team license × 1", total: 89, status: "Paid", date: "Sep 25" },
  ];

  let filter = $state<"All" | OrderStatus>("All");

  const visible = $derived(
    filter === "All" ? orders : orders.filter((o) => o.status === filter),
  );

  const revenue = $derived(
    orders.filter((o) => o.status === "Paid").reduce((sum, o) => sum + o.total, 0),
  );
  const pendingTotal = $derived(
    orders.filter((o) => o.status === "Pending").reduce((sum, o) => sum + o.total, 0),
  );
  const refundedCount = $derived(orders.filter((o) => o.status === "Refunded").length);

  const badgeClass: Record<OrderStatus, string> = {
    Paid: "ok",
    Pending: "warn",
    Refunded: "danger",
  };

  function money(value: number): string {
    return `$${value.toLocaleString("en-US")}`;
  }
</script>

<svelte:head>
  <title>Orders — GoWEZ Sample</title>
</svelte:head>

<div class="page-head">
  <div>
    <h1>Orders</h1>
    <p class="subtitle">
      A second data table with a local status filter — seeded orders, no backend involved.
    </p>
  </div>
  <div class="page-actions">
    <div class="filter">
      <label for="order-filter">Status</label>
      <select id="order-filter" bind:value={filter}>
        <option value="All">All</option>
        <option value="Paid">Paid</option>
        <option value="Pending">Pending</option>
        <option value="Refunded">Refunded</option>
      </select>
    </div>
  </div>
</div>

<section class="stats" aria-label="Order metrics">
  <div class="card stat">
    <span class="stat-label">Revenue</span>
    <span class="stat-value">{money(revenue)}</span>
    <span class="stat-hint">paid orders only</span>
  </div>
  <div class="card stat">
    <span class="stat-label">Pending</span>
    <span class="stat-value">{money(pendingTotal)}</span>
    <span class="stat-hint">awaiting payment</span>
  </div>
  <div class="card stat">
    <span class="stat-label">Refunded</span>
    <span class="stat-value">{refundedCount}</span>
    <span class="stat-hint">orders this period</span>
  </div>
</section>

<section class="card" aria-label="Orders table">
  <div class="section-head">
    <h2 class="section-title">Recent orders</h2>
    <span class="muted">{visible.length} of {orders.length} shown</span>
  </div>

  <div class="table-wrap">
    <table>
      <thead>
        <tr>
          <th scope="col">Order</th>
          <th scope="col">Customer</th>
          <th scope="col">Item</th>
          <th scope="col">Total</th>
          <th scope="col">Status</th>
          <th scope="col">Date</th>
        </tr>
      </thead>
      <tbody>
        {#each visible as order (order.id)}
          <tr>
            <td class="mono">{order.id}</td>
            <td class="member-name">{order.customer}</td>
            <td>{order.item}</td>
            <td class="mono">{money(order.total)}</td>
            <td><span class="badge {badgeClass[order.status]}">{order.status}</span></td>
            <td class="muted">{order.date}</td>
          </tr>
        {/each}
      </tbody>
    </table>

    {#if visible.length === 0}
      <div class="empty">
        <p class="empty-title">No {filter.toLowerCase()} orders.</p>
        <p class="muted">Pick another status from the filter above.</p>
      </div>
    {/if}
  </div>
</section>

<style>
  .filter {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .filter label {
    font-size: 12.5px;
    font-weight: 650;
    color: var(--muted);
  }

  .filter select {
    height: 34px;
    padding: 0 10px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--input-bg);
    color: var(--text);
    font: inherit;
    font-size: 13px;
  }

  .mono {
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }
</style>
