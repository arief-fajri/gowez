import { mount } from "svelte";
import App from "./App.svelte";
import "./styles/ui.css";

const target = document.getElementById("app");
if (!target) {
  throw new Error("main.ts: #app element not found");
}

mount(App, { target });
