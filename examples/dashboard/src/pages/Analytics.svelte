<script lang="ts">
  import type { User } from "../lib/types";

  let { users }: { users: User[] } = $props();

  const weeks: { label: string; value: number }[] = [
    { label: "Aug 12", value: 18 },
    { label: "Aug 19", value: 26 },
    { label: "Aug 26", value: 22 },
    { label: "Sep 02", value: 35 },
    { label: "Sep 09", value: 31 },
    { label: "Sep 16", value: 44 },
    { label: "Sep 23", value: 39 },
    { label: "Sep 30", value: 52 },
  ];

  const maxWeek = Math.max(...weeks.map((w) => w.value));

  const roleStats = $derived(
    (["Admin", "Editor", "Viewer"] as const).map((role) => {
      const count = users.filter((u) => u.role === role).length;
      const pct = users.length === 0 ? 0 : Math.round((count / users.length) * 100);
      return { role, count, pct };
    }),
  );

  const insights = [
    { label: "Signup trend", value: "+12%", hint: "vs. previous 30 days", dir: "up" },
    { label: "Avg. session", value: "4m 12s", hint: "median across visitors", dir: "flat" },
    { label: "Churn risk", value: "2", hint: "accounts flagged for review", dir: "down" },
  ];
</script>

<svelte:head>
  <title>Analytics — GoWEZ Sample</title>
</svelte:head>

<div class="page-head">
  <div>
    <h1>Analytics</h1>
    <p class="subtitle">
      Static charts and derived role distribution — no chart library, just CSS bars over sample
      data.
    </p>
  </div>
</div>

<section class="card" aria-label="Signups per week">
  <div class="section-head">
    <h2 class="section-title">Signups per week</h2>
    <span class="muted">last 8 weeks</span>
  </div>
  <div class="chart">
    {#each weeks as week (week.label)}
      <div class="bar-col">
        <span class="bar-value">{week.value}</span>
        <div
          class="bar"
          style="height: {Math.round((week.value / maxWeek) * 140)}px"
          title="{week.label}: {week.value} signups"
        ></div>
        <span class="bar-label">{week.label}</span>
      </div>
    {/each}
  </div>
</section>

<section class="split">
  <div class="card" aria-label="Role distribution">
    <div class="section-head">
      <h2 class="section-title">Role distribution</h2>
      <span class="muted">derived from the directory</span>
    </div>
    <ul class="dist">
      {#each roleStats as row (row.role)}
        <li>
          <div class="dist-head">
            <span class="dist-role">{row.role}</span>
            <span class="muted">{row.count} · {row.pct}%</span>
          </div>
          <div class="dist-track">
            <div class="dist-fill" style="width: {row.pct}%"></div>
          </div>
        </li>
      {/each}
    </ul>
  </div>

  <div class="card" aria-label="Insights">
    <div class="section-head">
      <h2 class="section-title">Insights</h2>
    </div>
    <ul class="insights">
      {#each insights as item (item.label)}
        <li>
          <div>
            <div class="insight-label">{item.label}</div>
            <div class="muted">{item.hint}</div>
          </div>
          <span class="insight-value {item.dir}">{item.value}</span>
        </li>
      {/each}
    </ul>
  </div>
</section>

<style>
  .chart {
    display: flex;
    align-items: flex-end;
    gap: 12px;
    height: 200px;
    padding-top: 8px;
  }

  .bar-col {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: flex-end;
    gap: 6px;
    min-width: 0;
  }

  .bar {
    width: 100%;
    max-width: 48px;
    border-radius: 6px 6px 0 0;
    background: var(--accent);
    transition: background-color 0.15s ease;
  }

  .bar:hover {
    background: var(--accent-hover);
  }

  .bar-value {
    font-size: 11.5px;
    font-weight: 650;
    font-variant-numeric: tabular-nums;
    color: var(--muted);
  }

  .bar-label {
    font-size: 10.5px;
    color: var(--muted);
    white-space: nowrap;
  }

  .dist {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .dist-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 6px;
  }

  .dist-role {
    font-size: 13.5px;
    font-weight: 600;
  }

  .dist-track {
    height: 10px;
    border-radius: 999px;
    background: var(--surface-2);
    overflow: hidden;
  }

  .dist-fill {
    height: 100%;
    border-radius: 999px;
    background: var(--accent);
    transition: width 0.3s ease;
  }

  .insights {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  .insights li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 10px 0;
    border-bottom: 1px solid var(--border);
  }

  .insights li:last-child {
    border-bottom: none;
  }

  .insight-label {
    font-size: 13.5px;
    font-weight: 600;
  }

  .insight-value {
    font-size: 15px;
    font-weight: 700;
    font-variant-numeric: tabular-nums;
  }

  .insight-value.up {
    color: var(--ok);
  }

  .insight-value.down {
    color: var(--danger);
  }

  .insight-value.flat {
    color: var(--muted);
  }
</style>
