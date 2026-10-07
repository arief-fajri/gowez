<script lang="ts">
  import type { NewUser, User } from "../lib/types";

  let {
    users,
    query,
    onAdd,
    onRemove,
    onToggle,
  }: {
    users: User[];
    query: string;
    onAdd: (user: NewUser) => void;
    onRemove: (user: User) => void;
    onToggle: (user: User) => void;
  } = $props();

  let name = $state("");
  let email = $state("");
  let role = $state<string>("Editor");
  let status = $state<string>("Active");
  let error = $state("");

  const filtered = $derived(
    query.trim() === ""
      ? users
      : users.filter((u) => {
          const q = query.trim().toLowerCase();
          return (
            u.name.toLowerCase().includes(q) ||
            u.email.toLowerCase().includes(q) ||
            u.role.toLowerCase().includes(q) ||
            u.status.toLowerCase().includes(q)
          );
        }),
  );

  function handleSubmit(event: SubmitEvent) {
    event.preventDefault();
    const trimmedName = name.trim();
    const trimmedEmail = email.trim();

    if (trimmedName === "") {
      error = "Full name is required.";
      return;
    }
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(trimmedEmail)) {
      error = "Enter a valid email address.";
      return;
    }
    if (users.some((u) => u.email.toLowerCase() === trimmedEmail.toLowerCase())) {
      error = "That email already exists in the directory.";
      return;
    }

    error = "";
    onAdd({ name: trimmedName, email: trimmedEmail, role: role as NewUser["role"], status: status as NewUser["status"] });
    name = "";
    email = "";
    role = "Editor";
    status = "Active";
  }

  function focusForm() {
    document.getElementById("field-name")?.focus();
  }
</script>

<svelte:head>
  <title>Users — GoWEZ Sample</title>
</svelte:head>

<div class="page-head">
  <div>
    <h1>Users</h1>
    <p class="subtitle">
      The member directory: search from the header, edit status inline, or add a new account with
      the form.
    </p>
  </div>
  <div class="page-actions">
    <button class="btn btn-primary" onclick={focusForm}>New user</button>
  </div>
</div>

<section class="card" aria-label="User directory">
  <div class="section-head">
    <h2 class="section-title">Member directory</h2>
    <span class="muted">{filtered.length} of {users.length} shown</span>
  </div>

  <div class="table-wrap">
    <table>
      <thead>
        <tr>
          <th scope="col">Member</th>
          <th scope="col">Role</th>
          <th scope="col">Status</th>
          <th scope="col" class="col-actions">Actions</th>
        </tr>
      </thead>
      <tbody>
        {#each filtered as user (user.id)}
          <tr>
            <td>
              <div class="member-name">{user.name}</div>
              <div class="member-email">{user.email}</div>
            </td>
            <td>{user.role}</td>
            <td>
              <span class="badge {user.status === 'Active' ? 'ok' : 'danger'}">{user.status}</span>
            </td>
            <td class="col-actions">
              <button class="btn btn-ghost" onclick={() => onToggle(user)}>
                {user.status === "Active" ? "Suspend" : "Activate"}
              </button>
              <button class="btn btn-ghost danger" onclick={() => onRemove(user)}>Remove</button>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>

    {#if filtered.length === 0}
      <div class="empty">
        <p class="empty-title">No members match “{query}”.</p>
        <p class="muted">Clear the search or add a new user from the form below.</p>
      </div>
    {/if}
  </div>
</section>

<section class="split">
  <div class="card" aria-label="Directory policy">
    <div class="section-head">
      <h2 class="section-title">Directory policy</h2>
    </div>
    <p class="policy-text">
      Accounts are seeded locally and never leave this page — there is no backend. <strong>Admin</strong>
      can manage billing and other admins, <strong>Editor</strong> can publish content, and <strong
        >Viewer</strong
      > has read-only access. Suspending an account keeps its history but blocks sign-in until it is
      reactivated from the table above.
    </p>
    <p class="policy-text">
      The form validates name, email shape, and duplicates before anything reaches the list, so
      every failure path (empty input, malformed email, collision) is observable as an explicit
      inline error.
    </p>
  </div>

  <div class="card" aria-label="Add user">
    <div class="section-head">
      <h2 class="section-title">Add user</h2>
    </div>
    <form class="form" onsubmit={handleSubmit} novalidate>
      <div class="field">
        <label for="field-name">Full name</label>
        <input id="field-name" type="text" placeholder="Jane Doe" bind:value={name} />
      </div>
      <div class="field">
        <label for="field-email">Email</label>
        <input id="field-email" type="email" placeholder="jane@gowez.dev" bind:value={email} />
      </div>
      <div class="form-row">
        <div class="field">
          <label for="field-role">Role</label>
          <select id="field-role" bind:value={role}>
            <option value="Admin">Admin</option>
            <option value="Editor">Editor</option>
            <option value="Viewer">Viewer</option>
          </select>
        </div>
        <div class="field">
          <label for="field-status">Status</label>
          <select id="field-status" bind:value={status}>
            <option value="Active">Active</option>
            <option value="Suspended">Suspended</option>
          </select>
        </div>
      </div>

      {#if error}
        <p class="form-error" role="alert">{error}</p>
      {/if}

      <button class="btn btn-primary btn-block" type="submit">Add to directory</button>
    </form>
  </div>
</section>

<style>
  .policy-text {
    margin: 10px 0 0;
    font-size: 13.5px;
    line-height: 1.65;
    color: var(--muted);
  }

  .policy-text strong {
    color: var(--text);
    font-weight: 650;
  }
</style>
