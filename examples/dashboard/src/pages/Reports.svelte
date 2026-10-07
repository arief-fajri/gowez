<script lang="ts">
  let { onNotify }: { onNotify: (message: string) => void } = $props();

  const reports = [
    {
      id: "rev-q3",
      title: "Q3 revenue",
      desc: "Quarterly revenue breakdown by plan, region, and channel with month-over-month deltas.",
      updated: "Oct 03, 2026",
      size: "1.2 MB",
    },
    {
      id: "activation",
      title: "Activation funnel",
      desc: "Signup → activation → first-value conversion for the last six release cycles.",
      updated: "Sep 28, 2026",
      size: "840 KB",
    },
    {
      id: "churn",
      title: "Churn cohort analysis",
      desc: "Retention curves per acquisition cohort with suspended-account annotations.",
      updated: "Sep 21, 2026",
      size: "2.1 MB",
    },
    {
      id: "api-usage",
      title: "API usage digest",
      desc: "Endpoint traffic, p95 latency, and permission denials grouped by application.",
      updated: "Sep 14, 2026",
      size: "560 KB",
    },
  ];

  function download(title: string) {
    onNotify(`"${title}" — download is not available in the sample.`);
  }
</script>

<svelte:head>
  <title>Reports — GoWEZ Sample</title>
</svelte:head>

<div class="page-head">
  <div>
    <h1>Reports</h1>
    <p class="subtitle">
      A static listing of report cards — the download action reports back through the shared flash
      banner.
    </p>
  </div>
</div>

<section class="report-grid" aria-label="Reports">
  {#each reports as report (report.id)}
    <article class="card report">
      <div class="report-top">
        <h2 class="section-title">{report.title}</h2>
        <span class="muted">{report.size}</span>
      </div>
      <p class="report-desc">{report.desc}</p>
      <div class="report-foot">
        <span class="muted">Updated {report.updated}</span>
        <button class="btn btn-ghost" onclick={() => download(report.title)}>Download</button>
      </div>
    </article>
  {/each}
</section>

<section class="card" aria-label="Report schedule">
  <div class="section-head">
    <h2 class="section-title">Schedule</h2>
    <span class="muted">static text</span>
  </div>
  <p class="schedule-text">
    Reports are generated nightly at 02:00 UTC and retained for 90 days. Digests are delivered to
    account admins according to the notification preference set on the Settings page. This section
    is deliberately static: it gives the runtime a long-form text block to shape and wrap next to
    interactive controls.
  </p>
</section>

<style>
  .report-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
    gap: 16px;
  }

  .report {
    display: flex;
    flex-direction: column;
  }

  .report-top {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
  }

  .report-desc {
    margin: 10px 0 0;
    font-size: 13px;
    line-height: 1.6;
    color: var(--muted);
    flex: 1;
  }

  .report-foot {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    margin-top: 16px;
    padding-top: 12px;
    border-top: 1px solid var(--border);
  }

  .schedule-text {
    margin: 10px 0 0;
    font-size: 13.5px;
    line-height: 1.65;
    color: var(--muted);
  }
</style>
