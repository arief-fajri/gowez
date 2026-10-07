<script lang="ts">
  import type { Activity, Slug, User } from "../lib/types";

  let {
    users,
    activity,
    onNavigate,
    onReset,
  }: {
    users: User[];
    activity: Activity[];
    onNavigate: (slug: Slug) => void;
    onReset: () => void;
  } = $props();

  const total = $derived(users.length);
  const activeCount = $derived(users.filter((u) => u.status === "Active").length);
  const suspendedCount = $derived(users.filter((u) => u.status === "Suspended").length);
  const recent = $derived(users.slice(0, 3));
</script>

<svelte:head>
  <title>Dashboard — GoWEZ Sample</title>
</svelte:head>

<div class="page-head">
  <div>
    <h1>Dashboard</h1>
    <p class="subtitle">
      Overview of the sample directory, live activity, and this browser test bed.
    </p>
  </div>
  <div class="page-actions">
    <button class="btn" onclick={onReset}>Reset demo</button>
    <button class="btn btn-primary" onclick={() => onNavigate("users")}>Manage users</button>
  </div>
</div>

<section class="stats" aria-label="Key metrics">
  <div class="card stat">
    <span class="stat-label">Total users</span>
    <span class="stat-value">{total}</span>
    <span class="stat-hint">directory entries</span>
  </div>
  <div class="card stat">
    <span class="stat-label">Active</span>
    <span class="stat-value">{activeCount}</span>
    <span class="stat-hint">currently allowed</span>
  </div>
  <div class="card stat">
    <span class="stat-label">Suspended</span>
    <span class="stat-value">{suspendedCount}</span>
    <span class="stat-hint">needs review</span>
  </div>
  <div class="card stat">
    <span class="stat-label">Events</span>
    <span class="stat-value">{activity.length}</span>
    <span class="stat-hint">captured this session</span>
  </div>
</section>

<section class="split">
  <div class="card" aria-label="Activity">
    <div class="section-head">
      <h2 class="section-title">Activity</h2>
      <span class="muted">live log</span>
    </div>
    <ul class="activity">
      {#each activity as entry (entry.id)}
        <li>
          <span class="dot {entry.kind}"></span>
          <span class="activity-text">{entry.text}</span>
          <span class="time">{entry.time}</span>
        </li>
      {/each}
    </ul>
  </div>

  <div class="card" aria-label="Recent members">
    <div class="section-head">
      <h2 class="section-title">Recent members</h2>
      <button class="btn btn-ghost" onclick={() => onNavigate("users")}>View all</button>
    </div>
    <ul class="recent">
      {#each recent as user (user.id)}
        <li>
          <div>
            <div class="member-name">{user.name}</div>
            <div class="member-email">{user.email}</div>
          </div>
          <span class="badge {user.status === 'Active' ? 'ok' : 'danger'}">{user.status}</span>
        </li>
      {/each}
    </ul>
  </div>
</section>

<section class="card about" aria-label="About this sample">
  <div class="section-head">
    <h2 class="section-title">About this sample</h2>
  </div>
  <p>
    This dashboard is the GoWEZ acceptance sample. It is intentionally a pure web application:
    what it renders here in the browser defines the feature set the GoWEZ runtime has to support
    when the Svelte compile pipeline lands. Anything the runtime cannot express yet — grid layout,
    scrollable regions, hash routing, two-way form binding, input/change/submit events — shows up
    as a concrete, testable gap instead of a silent misrender.
  </p>
  <p>
    The layout targets a 960 × 720 window (240 px sidebar + 720 px content area) and scales to
    wider viewports. Pages are addressable via hash URLs (<code>#/users</code>, <code>#/orders</code>,
    …) so browser back, forward, and refresh all keep their place.
  </p>
  <ul>
    <li>Six routed pages with real content switching</li>
    <li>Data table with row actions, search, and empty state</li>
    <li>Live activity listing fed by user actions</li>
    <li>Validated forms with two-way binding</li>
    <li>Light/dark theming and a state counter</li>
  </ul>
</section>

<style>
  .activity {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  .activity li {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 0;
    border-bottom: 1px solid var(--border);
    font-size: 13.5px;
  }

  .activity li:last-child {
    border-bottom: none;
  }

  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--accent);
    flex-shrink: 0;
  }

  .dot.add {
    background: var(--ok);
  }

  .dot.remove {
    background: var(--danger);
  }

  .dot.toggle {
    background: var(--accent);
  }

  .dot.system {
    background: var(--muted);
  }

  .activity-text {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .time {
    margin-left: auto;
    font-size: 12px;
    color: var(--muted);
    font-variant-numeric: tabular-nums;
    flex-shrink: 0;
  }

  .recent {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  .recent li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    padding: 10px 0;
    border-bottom: 1px solid var(--border);
  }

  .recent li:last-child {
    border-bottom: none;
  }

  .about p {
    margin: 10px 0 0;
    font-size: 13.5px;
    line-height: 1.65;
    color: var(--muted);
  }

  .about ul {
    margin: 12px 0 0;
    padding-left: 20px;
    font-size: 13.5px;
    line-height: 1.8;
    color: var(--muted);
  }

  .about code {
    padding: 1px 5px;
    border-radius: 5px;
    background: var(--surface-2);
    color: var(--text);
    font-size: 12.5px;
  }
</style>
