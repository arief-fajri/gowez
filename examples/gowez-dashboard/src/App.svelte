<script>
  import UserList from './UserList.svelte';

  // M5 acceptance slice: every construct here passes strict mode. Anything
  // outside the subset is listed in docs/SVELTE.md §Gap register.
  let count = $state(0);
  let query = $state('');
  let active = $state('dashboard');

  const sections = ['dashboard', 'users', 'reports'];

  let users = $state([
    { id: 1, name: 'Aria', role: 'admin' },
    { id: 2, name: 'Bagus', role: 'editor' },
    { id: 3, name: 'Citra', role: 'editor' },
  ]);

  let filtered = $derived(
    query === ''
      ? users
      : users.filter(function (u) {
          return u.name.toLowerCase().indexOf(query.toLowerCase()) >= 0;
        }),
  );

  function inc() {
    count += 1;
  }

  function dec() {
    count -= 1;
  }

  function onRemove(id) {
    users = users.filter(function (u) {
      return u.id !== id;
    });
  }
</script>

<section class="shell">
  <header class="bar">
    <p class="brand">GoWEZ</p>
    <nav>
      {#each sections as s (s)}
        <button class:active={active === s} onclick={function () { active = s; }}>{s}</button>
      {/each}
    </nav>
  </header>

  <div class="body">
    <aside class="side">
      <p class="label">counter</p>
      <p class="count">{count}</p>
      <button onclick={dec}>-1</button>
      <button onclick={inc}>+1</button>
      {#if count > 0}
        <p class="hint">positive</p>
      {:else}
        <p class="hint">zero or below</p>
      {/if}
    </aside>

    <main class="main">
      <input bind:value={query} placeholder="filter users" />
      <p class="label">search: {query}</p>
      <UserList {users} items={filtered} onRemove={onRemove} />
      {#if filtered.length === 0}
        <p class="hint">no users match</p>
      {/if}
    </main>
  </div>
</section>

<style>
  /* An explicit height: the CSS subset has no percentage heights, and flex-grow needs a
     bounded parent to grow into.
     The shell colour is deliberately NOT the runtime's scene backdrop (#14161c): an
     identical colour hides the panel's own bounds, so a layout bug is invisible. */
  shell { display: flex; flex-direction: column; height: 600px; background-color: #1b1f27; color: #dfe4ee; }
  bar { display: flex; align-items: center; gap: 12px; padding: 10px; background-color: #1e2028; }
  brand { color: #ffffff; font-size: 20px; }
  nav { display: flex; gap: 6px; }
  body { display: flex; flex-grow: 1; }
  side { display: flex; flex-direction: column; gap: 6px; padding: 12px; background-color: #1a1c24; }
  main { display: flex; flex-direction: column; gap: 8px; padding: 12px; }
  input { height: 24px; background-color: #202430; color: #dfe4ee; padding: 4px; border-width: 1px; border-color: #333945; }
  label { color: #8b93a7; font-size: 12px; }
  count { color: #e0873a; font-size: 28px; }
  hint { color: #6fcf97; font-size: 12px; }
  button { background-color: #2b303b; color: #ffffff; padding: 6px 10px; }
</style>
