<script>
  // Child component: composition by props and a callback prop. No slots, no
  // snippets — the subset's replacement for them (docs/SVELTE.md).
  let { users = [], items = [], onRemove = function () {} } = $props();
</script>

<ul class="list">
  {#each items as user (user.id)}
    <li class="row">
      <p class="name">{user.name}</p>
      <p class="role">{user.role}</p>
      <button onclick={function () { onRemove(user.id); }}>remove</button>
    </li>
  {/each}
</ul>

<style>
  list { display: flex; flex-direction: column; gap: 4px; }
  row { display: flex; align-items: center; gap: 8px; padding: 4px; background-color: #202430; }
  name { color: #ffffff; font-size: 14px; }
  role { color: #8b93a7; font-size: 12px; }
  button { background-color: #2b303b; color: #ffffff; padding: 4px 8px; }
</style>
