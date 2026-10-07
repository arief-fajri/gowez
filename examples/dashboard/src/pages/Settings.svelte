<script lang="ts">
  import type { Theme } from "../lib/types";

  let {
    theme,
    onThemeChange,
    onReset,
    onNotify,
  }: {
    theme: Theme;
    onThemeChange: (theme: Theme) => void;
    onReset: () => void;
    onNotify: (message: string) => void;
  } = $props();

  let displayName = $state("Ada Lovelace");
  let defaultRole = $state("Editor");
  let notifyMode = $state("Digest");
  let error = $state("");

  function handleSave(event: SubmitEvent) {
    event.preventDefault();
    if (displayName.trim() === "") {
      error = "Display name is required.";
      return;
    }
    error = "";
    onNotify("Preferences saved (local sample state).");
  }

  function handleTheme(event: Event) {
    const value = (event.currentTarget as HTMLSelectElement).value;
    onThemeChange(value as Theme);
  }
</script>

<svelte:head>
  <title>Settings — GoWEZ Sample</title>
</svelte:head>

<div class="page-head">
  <div>
    <h1>Settings</h1>
    <p class="subtitle">
      Preferences form with two-way binding — the theme select stays in sync with the header
      toggle.
    </p>
  </div>
</div>

<section class="split">
  <div class="card" aria-label="Preferences">
    <div class="section-head">
      <h2 class="section-title">Preferences</h2>
    </div>
    <form class="form" onsubmit={handleSave} novalidate>
      <div class="field">
        <label for="settings-name">Display name</label>
        <input id="settings-name" type="text" bind:value={displayName} />
      </div>
      <div class="form-row">
        <div class="field">
          <label for="settings-role">Default role</label>
          <select id="settings-role" bind:value={defaultRole}>
            <option value="Admin">Admin</option>
            <option value="Editor">Editor</option>
            <option value="Viewer">Viewer</option>
          </select>
        </div>
        <div class="field">
          <label for="settings-notify">Notifications</label>
          <select id="settings-notify" bind:value={notifyMode}>
            <option value="Off">Off</option>
            <option value="Digest">Daily digest</option>
            <option value="Instant">Instant</option>
          </select>
        </div>
      </div>
      <div class="field">
        <label for="settings-theme">Theme</label>
        <select id="settings-theme" value={theme} onchange={handleTheme}>
          <option value="light">Light</option>
          <option value="dark">Dark</option>
        </select>
      </div>

      {#if error}
        <p class="form-error" role="alert">{error}</p>
      {/if}

      <button class="btn btn-primary" type="submit">Save preferences</button>
    </form>
  </div>

  <div class="card" aria-label="Sample data">
    <div class="section-head">
      <h2 class="section-title">Sample data</h2>
    </div>
    <p class="side-text">
      Everything in this app lives in memory. Resetting restores the seed directory, clears the
      activity log, search, counter, and the add-user form.
    </p>
    <button class="btn danger-zone" onclick={onReset}>Reset demo data</button>
    <p class="side-note">GoWEZ Sample · v0.1.0 · pure-web test bed for Milestone 5.</p>
  </div>
</section>

<style>
  .side-text {
    margin: 10px 0 16px;
    font-size: 13.5px;
    line-height: 1.6;
    color: var(--muted);
  }

  .danger-zone {
    width: 100%;
    border-color: color-mix(in srgb, var(--danger) 45%, transparent);
    color: var(--danger);
  }

  .danger-zone:hover {
    background: var(--danger-bg);
  }

  .side-note {
    margin: 16px 0 0;
    font-size: 12px;
    color: var(--muted);
  }
</style>
