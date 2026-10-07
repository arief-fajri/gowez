<!--
  Dashboard sample — runnable Svelte 5 app (browser test bed for GoWEZ).

  Pure web app on purpose: it exercises capabilities the GoWEZ runtime must
  eventually cover (grid, scroll, hash routing, bind:value, input/change/submit
  events, theming, list + conditional rendering) and is not bound by the M2 CSS
  subset. Run: npm run dev -w @gowez/example-dashboard

  This file is the shell: header, sidebar, shared state, and the hash router.
  Page content lives in src/pages/*.svelte.
-->
<script lang="ts">
  import { onMount } from "svelte";
  import { fade } from "svelte/transition";
  import Dashboard from "./pages/Dashboard.svelte";
  import Users from "./pages/Users.svelte";
  import Analytics from "./pages/Analytics.svelte";
  import Orders from "./pages/Orders.svelte";
  import Reports from "./pages/Reports.svelte";
  import Settings from "./pages/Settings.svelte";
  import type { Activity, Kind, NewUser, Role, Slug, Status, Theme, User } from "./lib/types";

  const seedUsers: User[] = [
    { id: 1, name: "Alice Nakamura", email: "alice@gowez.dev", role: "Admin", status: "Active" },
    { id: 2, name: "Bruno Silva", email: "bruno@gowez.dev", role: "Editor", status: "Active" },
    { id: 3, name: "Chen Wei", email: "chen@gowez.dev", role: "Viewer", status: "Suspended" },
    { id: 4, name: "Dana Haryanto", email: "dana@gowez.dev", role: "Editor", status: "Active" },
    { id: 5, name: "Elif Demir", email: "elif@gowez.dev", role: "Viewer", status: "Active" },
  ];

  const seedActivity: Activity[] = [
    { id: 1, text: "System backup completed", time: "09:12", kind: "system" },
    { id: 2, text: "Elif Demir signed in", time: "08:47", kind: "system" },
    { id: 3, text: "Q3 revenue report exported", time: "08:20", kind: "system" },
  ];

  const navSections: { label: string; items: { slug: Slug; label: string }[] }[] = [
    {
      label: "Overview",
      items: [
        { slug: "dashboard", label: "Dashboard" },
        { slug: "analytics", label: "Analytics" },
      ],
    },
    {
      label: "Management",
      items: [
        { slug: "users", label: "Users" },
        { slug: "orders", label: "Orders" },
        { slug: "reports", label: "Reports" },
      ],
    },
    { label: "System", items: [{ slug: "settings", label: "Settings" }] },
  ];

  const validSlugs: Slug[] = ["dashboard", "users", "analytics", "orders", "reports", "settings"];

  function slugFromHash(hash: string): Slug {
    const raw = hash.replace(/^#\/?/, "").toLowerCase();
    return (validSlugs as string[]).includes(raw) ? (raw as Slug) : "dashboard";
  }

  let theme = $state<Theme>("light");
  let count = $state(0);
  let query = $state("");
  let flash = $state("");
  let page = $state<Slug>(slugFromHash(location.hash));

  let users = $state<User[]>(seedUsers.map((u) => ({ ...u })));
  let activity = $state<Activity[]>(seedActivity.map((a) => ({ ...a })));

  let nextId = 100;
  let flashTimer: ReturnType<typeof setTimeout> | undefined;

  onMount(() => {
    const onHash = () => {
      const slug = slugFromHash(location.hash);
      if (location.hash !== `#/${slug}`) {
        location.replace(`#/${slug}`);
        return;
      }
      page = slug;
    };
    window.addEventListener("hashchange", onHash);
    if (!location.hash) {
      location.replace("#/dashboard");
    } else {
      onHash();
    }
    return () => window.removeEventListener("hashchange", onHash);
  });

  function navigate(slug: Slug) {
    if (location.hash === `#/${slug}`) {
      page = slug;
      return;
    }
    location.hash = `#/${slug}`;
  }

  function now(): string {
    return new Date().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  }

  function log(text: string, kind: Kind) {
    activity = [{ id: nextId++, text, time: now(), kind }, ...activity].slice(0, 8);
  }

  function showFlash(message: string) {
    flash = message;
    if (flashTimer) clearTimeout(flashTimer);
    flashTimer = setTimeout(() => (flash = ""), 3000);
  }

  function addUser(data: NewUser) {
    users = [{ id: nextId++, ...data }, ...users];
    log(`Added ${data.name}`, "add");
    showFlash(`${data.name} added to the directory.`);
  }

  function removeUser(user: User) {
    users = users.filter((u) => u.id !== user.id);
    log(`Removed ${user.name}`, "remove");
    showFlash(`${user.name} removed.`);
  }

  function toggleStatus(user: User) {
    const next: Status = user.status === "Active" ? "Suspended" : "Active";
    users = users.map((u) => (u.id === user.id ? { ...u, status: next } : u));
    log(`${next === "Active" ? "Reactivated" : "Suspended"} ${user.name}`, "toggle");
  }

  function resetDemo() {
    users = seedUsers.map((u) => ({ ...u }));
    activity = seedActivity.map((a) => ({ ...a }));
    query = "";
    count = 0;
    showFlash("Demo data reset to its seed state.");
  }

  function toggleTheme() {
    theme = theme === "light" ? "dark" : "light";
  }
</script>

<div class="app {theme}">
  <header class="topbar">
    <div class="brand">
      <span class="brand-mark">G</span>
      <span class="brand-name">GoWEZ Console</span>
    </div>

    {#if page === "users"}
      <input
        class="search"
        type="search"
        placeholder="Search members, roles, status…"
        aria-label="Search directory"
        bind:value={query}
      />
    {/if}

    <div class="top-actions">
      <div class="counter" role="group" aria-label="Sample counter">
        <button class="icon-btn" onclick={() => (count -= 1)} aria-label="Decrease">−</button>
        <span class="counter-value">{count}</span>
        <button class="icon-btn" onclick={() => (count += 1)} aria-label="Increase">+</button>
      </div>
      <button class="btn" onclick={toggleTheme}>Theme: {theme === "light" ? "Light" : "Dark"}</button>
      <span class="avatar" aria-hidden="true">AD</span>
    </div>
  </header>

  <aside class="sidebar">
    <nav aria-label="Main navigation">
      {#each navSections as section (section.label)}
        <div class="nav-section">
          <span class="nav-label">{section.label}</span>
          {#each section.items as item (item.slug)}
            <button
              class="nav-item"
              class:active={page === item.slug}
              aria-current={page === item.slug ? "page" : undefined}
              onclick={() => navigate(item.slug)}
            >
              {item.label}
            </button>
          {/each}
        </div>
      {/each}
    </nav>
    <footer class="side-foot">v0.1.0 · M5 test bed</footer>
  </aside>

  <main class="main">
    <div class="stack">
      {#if flash}
        <div class="flash" role="status">
          <span>{flash}</span>
          <button class="flash-close" onclick={() => (flash = "")} aria-label="Dismiss">✕</button>
        </div>
      {/if}

      {#key page}
        <div class="page" in:fade={{ duration: 140 }}>
          {#if page === "dashboard"}
            <Dashboard {users} {activity} onNavigate={navigate} onReset={resetDemo} />
          {:else if page === "users"}
            <Users {users} {query} onAdd={addUser} onRemove={removeUser} onToggle={toggleStatus} />
          {:else if page === "analytics"}
            <Analytics {users} />
          {:else if page === "orders"}
            <Orders />
          {:else if page === "reports"}
            <Reports onNotify={showFlash} />
          {:else if page === "settings"}
            <Settings
              {theme}
              onThemeChange={(t) => (theme = t)}
              onReset={resetDemo}
              onNotify={showFlash}
            />
          {/if}
        </div>
      {/key}
    </div>
  </main>
</div>

<style>
  .app {
    --bg: #f3f5f9;
    --surface: #ffffff;
    --surface-2: #eef1f6;
    --input-bg: #ffffff;
    --text: #17202e;
    --muted: #64748b;
    --border: #dde3ec;
    --accent: #2f6feb;
    --accent-hover: #255ed1;
    --accent-soft: #e8f0fe;
    --ok: #16a34a;
    --ok-bg: #dcfce7;
    --warn: #b45309;
    --warn-bg: #fef3c7;
    --danger: #dc2626;
    --danger-bg: #fee2e2;
    --shadow: 0 1px 2px rgb(16 24 40 / 6%), 0 8px 24px rgb(16 24 40 / 6%);

    display: grid;
    grid-template-columns: 240px 1fr;
    grid-template-rows: 64px 1fr;
    height: 100dvh;
    background: var(--bg);
    color: var(--text);
    font-family:
      system-ui,
      -apple-system,
      "Segoe UI",
      Roboto,
      "Helvetica Neue",
      sans-serif;
    font-size: 14px;
    line-height: 1.45;
  }

  .app.dark {
    --bg: #0f1117;
    --surface: #171a23;
    --surface-2: #1e222e;
    --input-bg: #12151d;
    --text: #e6eaf2;
    --muted: #94a3b8;
    --border: #2a2f3d;
    --accent: #3b82f6;
    --accent-hover: #5b9aff;
    --accent-soft: #1c2a4a;
    --ok: #4ade80;
    --ok-bg: #12291d;
    --warn: #fbbf24;
    --warn-bg: #2a2110;
    --danger: #f87171;
    --danger-bg: #2b1518;
    --shadow: 0 1px 2px rgb(0 0 0 / 40%), 0 10px 30px rgb(0 0 0 / 35%);
  }

  .topbar {
    grid-column: 1 / -1;
    display: flex;
    align-items: center;
    gap: 16px;
    padding: 0 20px;
    background: var(--surface);
    border-bottom: 1px solid var(--border);
  }

  .brand {
    display: flex;
    align-items: center;
    gap: 10px;
    flex-shrink: 0;
  }

  .brand-mark {
    display: grid;
    place-items: center;
    width: 30px;
    height: 30px;
    border-radius: 8px;
    background: var(--accent);
    color: #ffffff;
    font-size: 15px;
    font-weight: 700;
  }

  .brand-name {
    font-weight: 650;
    font-size: 15px;
    white-space: nowrap;
  }

  .search {
    flex: 1;
    max-width: 420px;
    height: 36px;
    padding: 0 12px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--input-bg);
    color: var(--text);
    font: inherit;
    font-size: 13.5px;
    transition: border-color 0.15s ease, box-shadow 0.15s ease;
  }

  .search::placeholder {
    color: var(--muted);
  }

  .search:focus {
    outline: none;
    border-color: var(--accent);
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent) 25%, transparent);
  }

  .top-actions {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-left: auto;
  }

  .counter {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 3px;
    border: 1px solid var(--border);
    border-radius: 999px;
    background: var(--surface-2);
  }

  .counter-value {
    min-width: 30px;
    text-align: center;
    font-size: 13px;
    font-weight: 650;
    font-variant-numeric: tabular-nums;
  }

  .icon-btn {
    width: 26px;
    height: 26px;
    border: none;
    border-radius: 50%;
    background: var(--surface);
    color: var(--text);
    font-size: 14px;
    line-height: 1;
    cursor: pointer;
    transition: background-color 0.15s ease, color 0.15s ease;
  }

  .icon-btn:hover {
    background: var(--accent);
    color: #ffffff;
  }

  .avatar {
    display: grid;
    place-items: center;
    width: 32px;
    height: 32px;
    border-radius: 50%;
    background: var(--accent);
    color: #ffffff;
    font-size: 12px;
    font-weight: 700;
  }

  .sidebar {
    display: flex;
    flex-direction: column;
    gap: 16px;
    padding: 18px 14px;
    background: var(--surface);
    border-right: 1px solid var(--border);
    overflow-y: auto;
  }

  .nav-section {
    display: flex;
    flex-direction: column;
    gap: 2px;
    margin-bottom: 16px;
  }

  .nav-label {
    margin: 0 0 6px 10px;
    font-size: 11px;
    font-weight: 700;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--muted);
  }

  .nav-item {
    display: flex;
    width: 100%;
    padding: 9px 12px;
    border: none;
    border-radius: 8px;
    background: transparent;
    color: var(--muted);
    font: inherit;
    font-size: 14px;
    text-align: left;
    cursor: pointer;
    transition: background-color 0.15s ease, color 0.15s ease;
  }

  .nav-item:hover {
    background: var(--surface-2);
    color: var(--text);
  }

  .nav-item.active {
    background: var(--accent-soft);
    color: var(--accent);
    font-weight: 650;
  }

  .side-foot {
    margin-top: auto;
    font-size: 11.5px;
    color: var(--muted);
  }

  .main {
    overflow-y: auto;
    padding: 24px;
  }

  .stack {
    display: flex;
    flex-direction: column;
    gap: 16px;
    max-width: 1160px;
    margin: 0 auto;
  }

  .page {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  @media (max-width: 860px) {
    .app {
      grid-template-columns: 1fr;
      grid-template-rows: 64px auto 1fr;
    }

    .sidebar {
      flex-direction: row;
      align-items: center;
      gap: 12px;
      padding: 10px 14px;
      border-right: none;
      border-bottom: 1px solid var(--border);
      overflow-x: auto;
    }

    .sidebar nav {
      display: flex;
      align-items: center;
      gap: 14px;
    }

    .nav-section {
      flex-direction: row;
      align-items: center;
      gap: 4px;
      margin-bottom: 0;
    }

    .nav-label {
      margin: 0 4px 0 0;
    }

    .nav-item {
      width: auto;
      white-space: nowrap;
      padding: 7px 10px;
    }

    .side-foot {
      display: none;
    }
  }

  @media (max-width: 700px) {
    .search {
      display: none;
    }
  }
</style>
