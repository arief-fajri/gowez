<!--
  Counter example — Milestone 5.

  Svelte is the authoring layer only: this component compiles to GoWEZ UI
  instructions and runs on the Go runtime. No browser, no Chromium, no OS
  WebView. Behaviors outside the documented subset are rejected at compile
  time, not silently dropped.
-->
<script lang="ts">
  let count = $state(0);

  function increment() {
    count += 1;
  }
</script>

<button onclick={increment}>count: {count}</button>
