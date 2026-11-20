import { expect, mock, test } from "bun:test";

mock.module("lucide", () => ({
	ArrowUp: {}, Bot: {}, ChevronDown: {}, Copy: {}, LoaderCircle: {}, MessagesSquare: {}, NotebookPen: {}, Settings: {}, Sparkles: {},
	createIcons() {},
}));

class Element {
	constructor() {
		this.children = [];
		this.listeners = {};
		this.attributes = {};
		this.classes = new Set();
		this.classList = {
			toggle: (name, force) => {
				const enabled = force === undefined ? !this.classes.has(name) : force;
				if (enabled) this.classes.add(name);
				else this.classes.delete(name);
				return enabled;
			},
			add: (name) => this.classes.add(name),
			remove: (name) => this.classes.delete(name),
			contains: (name) => this.classes.has(name),
		};
		this.value = "";
		this.disabled = false;
		this.focused = false;
		this.scrollHeight = 0;
		this.scrollTop = 0;
		this.clientHeight = 0;
	}
	append(...children) { this.children.push(...children); }
	replaceChildren(...children) { this.children = children; }
	setAttribute(name, value) { this.attributes[name] = value; }
	addEventListener(name, callback) { this.listeners[name] = callback; }
	focus() { this.focused = true; }
	get options() { return this.children; }
}

test("submission clears the input and formats the pending bubble before the reply", async () => {
	const elements = new Map();
	globalThis.document = {
		querySelector(selector) {
			if (!elements.has(selector)) elements.set(selector, new Element());
			return elements.get(selector);
		},
		querySelectorAll: () => [],
		createElement: () => new Element(),
		createTextNode: (text) => ({ textContent: text }),
	};
	const sessionValues = new Map();
	globalThis.window = { sessionStorage: { getItem: (key) => sessionValues.get(key) ?? null, setItem: (key, value) => sessionValues.set(key, value), removeItem: (key) => sessionValues.delete(key) }, matchMedia: () => ({ matches: true }), addEventListener() {} };
	let finishSend;
	const messageRequests = [];
	const settingsSaves = [];
	const modelRequests = [];
	let activeState = { setup_required: false, profile: { slug: "artist", name: "Artist" }, model: "model-a", revision: 1, pending_notes: 0 };
	let delayedArtistModels = false;
	let resolveArtistModels;
	let failedModelsProfile = "";
	let failSave = false;
	let saveErrorCode = "settings_save_failed";
	let historyLoads = 0;
	let memoryLoads = 0;
	globalThis.fetch = async (url, options = {}) => {
		if (url === "/api/messages") {
			messageRequests.push(JSON.parse(options.body));
			return new Promise((resolve) => { finishSend = resolve; });
		}
		if (url.startsWith("/api/messages?")) {
			historyLoads++;
			return { ok: true, json: async () => ({ items: [], next_cursor: null }) };
		}
		if (url.startsWith("/api/memory?")) {
			memoryLoads++;
			return { ok: true, json: async () => ({ items: [], next_offset: null }) };
		}
		if (url === "/api/state") return { ok: true, json: async () => activeState };
		if (url === "/api/settings" && options.method === "POST") {
			const request = JSON.parse(options.body);
			settingsSaves.push(request);
			if (failSave) return { ok: false, status: saveErrorCode === "settings_changed" ? 409 : saveErrorCode === "settings_save_failed" ? 500 : 503, json: async () => ({ error: { message: "settings failed", code: saveErrorCode } }) };
			activeState = { ...activeState, profile: { slug: request.profile_slug, name: request.profile_slug === "beta" ? "Beta" : request.profile_slug }, model: request.model, setup_required: request.profile_slug === "gamma", revision: activeState.revision + 1 };
			return { ok: true, json: async () => activeState };
		}
		if (url === "/api/settings") return { ok: true, json: async () => ({ active_profile_slug: activeState.profile.slug, profiles: [{ slug: "artist", name: "Artist" }, { slug: "beta", name: "Beta" }, { slug: "gamma", name: "Gamma" }] }) };
		if (url.startsWith("/api/settings/models?")) {
			const profile = new URLSearchParams(url.split("?")[1]).get("profile");
			modelRequests.push(profile);
			if (failedModelsProfile === profile) return { ok: false, status: 502, json: async () => ({ error: { message: "model list failed", code: "models_unavailable" } }) };
			if (profile === "artist" && delayedArtistModels) {
				return new Promise((resolve) => { resolveArtistModels = resolve; });
			}
			const savedModel = profile === activeState.profile.slug ? activeState.model : profile === "artist" ? "model-a" : "beta-model-1";
			const body = profile === "gamma"
				? { profile_slug: profile, saved_model: "", models: [], setup_required: true }
				: { profile_slug: profile, saved_model: savedModel, models: profile === "artist" ? ["model-a"] : ["beta-model-1", "beta-model-2"], setup_required: false };
			return { ok: true, json: async () => body };
		}
		const body = url === "/api/markdown" ? { content_html: "<p>First</p>\n<p><strong>Second</strong></p>\n" } : { items: [], next_cursor: null, next_offset: null };
		return { ok: true, json: async () => body };
	};
	await import("./app.js");
	await Bun.sleep(0);
	const input = elements.get("#message-input");
	input.value = "First\n\n**Second**";
	elements.get("#message-form").listeners.submit({ preventDefault() {} });
	expect(input.value).toBe("");
	expect(messageRequests[0]?.profile_slug).toBe("artist");
	let row = elements.get("#message-list").children.find((child) => child.attributes?.["aria-label"] === "Your message");
	expect(row.children[0].children[1].classes.has("whitespace-pre-wrap")).toBe(true);
	expect(row.children[0].children[1].textContent).toBe("First\n\n**Second**");
	await Bun.sleep(0);
	row = elements.get("#message-list").children.find((child) => child.attributes?.["aria-label"] === "Your message");
	expect(row.children[0].children[1].innerHTML).toBe("<p>First</p>\n<p><strong>Second</strong></p>\n");
	input.value = "new draft";
	finishSend({ ok: false, status: 502, json: async () => ({ error: { message: "failed" } }) });
	await Bun.sleep(0);
	expect(input.value).toBe("new draft");

	elements.get("#message-form").listeners.submit({ preventDefault() {} });
	expect(input.value).toBe("");
	finishSend({ ok: false, status: 502, json: async () => ({ error: { message: "failed" } }) });
	await Bun.sleep(0);
	expect(input.value).toBe("new draft");

	elements.get("#message-form").listeners.submit({ preventDefault() {} });
	input.value = "new draft";
	finishSend({ ok: true, json: async () => ({}) });
	await Bun.sleep(0);
	expect(input.value).toBe("new draft");
	expect(Boolean(elements.get("#settings-button")?.listeners.click)).toBe(true);

	const settingsButton = elements.get("#settings-button");
	input.value = "unfinished turn";
	elements.get("#message-form").listeners.submit({ preventDefault() {} });
	await Bun.sleep(0);
	await settingsButton.listeners.click();
	expect(elements.get("#settings-profile").value).toBe("artist");
	expect(elements.get("#settings-model").value).toBe("model-a");
	expect(elements.get("#settings-panel").classList.contains("hidden")).toBe(false);
	expect(elements.get("#settings-save").disabled).toBe(true);
	elements.get("#settings-cancel").listeners.click();
	finishSend({ ok: true, json: async () => ({}) });
	await Bun.sleep(0);

	input.value = "draft preserved by cancel";
	delayedArtistModels = true;
	const opening = settingsButton.listeners.click();
	await Bun.sleep(0);
	await Bun.sleep(0);
	expect(typeof resolveArtistModels).toBe("function");
	const profileSelect = elements.get("#settings-profile");
	expect(profileSelect.disabled).toBe(false);
	profileSelect.value = "beta";
	await profileSelect.listeners.change();
	const modelSelect = elements.get("#settings-model");
	expect(modelSelect.value).toBe("beta-model-1");
	resolveArtistModels({ ok: true, json: async () => ({ profile_slug: "artist", saved_model: "model-a", models: ["model-a"], setup_required: false }) });
	await opening;
	expect(modelSelect.value).toBe("beta-model-1");
	profileSelect.value = "beta";
	elements.get("#settings-cancel").listeners.click();
	expect(settingsSaves.length).toBe(0);
	expect(activeState.profile.slug).toBe("artist");
	expect(input.value).toBe("draft preserved by cancel");
	expect(elements.get("#settings-panel").classList.contains("hidden")).toBe(true);
	expect(elements.get("#workspace").classList.contains("hidden")).toBe(false);
	expect(settingsButton.focused).toBe(true);

	delayedArtistModels = false;
	await settingsButton.listeners.click();
	profileSelect.value = "beta";
	await profileSelect.listeners.change();
	modelSelect.value = "beta-model-2";
	modelSelect.listeners.change();
	const oldHistoryLoads = historyLoads;
	const oldMemoryLoads = memoryLoads;
	const sentMessagesBeforeSave = messageRequests.length;
	input.value = "unsent prompt to preserve";
	await elements.get("#settings-form").listeners.submit({ preventDefault() {} });
	expect(settingsSaves).toHaveLength(1);
	expect(settingsSaves[0]).toEqual({ profile_slug: "beta", model: "beta-model-2", revision: 1 });
	expect(activeState.profile.slug).toBe("beta");
	expect(activeState.model).toBe("beta-model-2");
	expect(input.value).toBe("unsent prompt to preserve");
	expect(messageRequests).toHaveLength(sentMessagesBeforeSave);
	expect(historyLoads).toBeGreaterThan(oldHistoryLoads);
	expect(memoryLoads).toBeGreaterThan(oldMemoryLoads);
	expect(elements.get("#settings-panel").classList.contains("hidden")).toBe(true);
	expect(elements.get("#profile-name").textContent).toBe("Beta");

	failedModelsProfile = "beta";
	await settingsButton.listeners.click();
	expect(elements.get("#settings-panel").classList.contains("hidden")).toBe(false);
	expect(elements.get("#settings-save").disabled).toBe(true);
	expect(elements.get("#settings-status").children[0]?.textContent).toContain("Could not load models");
	expect(elements.get("#settings-retry-models").classList.contains("hidden")).toBe(false);
	failedModelsProfile = "";
	await elements.get("#settings-retry-models").listeners.click();
	expect(modelSelect.value).toBe("beta-model-2");
	expect(elements.get("#settings-save").disabled).toBe(false);
	elements.get("#settings-cancel").listeners.click();

	failSave = true;
	await settingsButton.listeners.click();
	profileSelect.value = "beta";
	await profileSelect.listeners.change();
	modelSelect.value = "beta-model-1";
	modelSelect.listeners.change();
	await elements.get("#settings-form").listeners.submit({ preventDefault() {} });
	expect(elements.get("#settings-panel").classList.contains("hidden")).toBe(false);
	expect(elements.get("#settings-status").children[0]?.textContent).toContain("settings failed");
	expect(activeState.model).toBe("beta-model-2");
	elements.get("#settings-cancel").listeners.click();
	failSave = false;
	saveErrorCode = "settings_changed";
	failSave = true;
	await settingsButton.listeners.click();
	profileSelect.value = "beta";
	await profileSelect.listeners.change();
	await elements.get("#settings-form").listeners.submit({ preventDefault() {} });
	expect(elements.get("#settings-save").disabled).toBe(true);
	expect(elements.get("#settings-status").children[0]?.textContent).toContain("changed in another tab");
	activeState = { ...activeState, model: "beta-model-1", revision: 3 };
	const reloadButton = elements.get("#settings-status").children.at(-1);
	await reloadButton.listeners.click();
	expect(profileSelect.value).toBe("beta");
	expect(modelSelect.value).toBe("beta-model-1");
	expect(elements.get("#settings-save").disabled).toBe(false);
	failSave = false;
	saveErrorCode = "settings_save_failed";
	modelSelect.value = "beta-model-2";
	modelSelect.listeners.change();
	await elements.get("#settings-form").listeners.submit({ preventDefault() {} });
	expect(settingsSaves.at(-1).revision).toBe(3);
	expect(activeState.revision).toBe(4);

	await settingsButton.listeners.click();
	profileSelect.value = "gamma";
	await profileSelect.listeners.change();
	expect(modelSelect.disabled).toBe(true);
	expect(modelSelect.options).toHaveLength(0);
	expect(elements.get("#settings-model-help").textContent).toContain("no configured provider");
	await elements.get("#settings-form").listeners.submit({ preventDefault() {} });
	expect(settingsSaves.at(-1)).toEqual({ profile_slug: "gamma", model: "", revision: 4 });
	expect(activeState.setup_required).toBe(true);
	expect(elements.get("#settings-panel").classList.contains("hidden")).toBe(true);

	await settingsButton.listeners.click();
	profileSelect.value = "beta";
	await profileSelect.listeners.change();
	await elements.get("#settings-form").listeners.submit({ preventDefault() {} });
	expect(activeState.setup_required).toBe(false);

	failSave = true;
	saveErrorCode = "settings_recovery_required";
	await settingsButton.listeners.click();
	profileSelect.value = "beta";
	await profileSelect.listeners.change();
	modelSelect.value = "beta-model-1";
	modelSelect.listeners.change();
	await elements.get("#settings-form").listeners.submit({ preventDefault() {} });
	expect(elements.get("#settings-panel").classList.contains("hidden")).toBe(false);
	expect(elements.get("#settings-status").children[0]?.textContent).toContain("settings failed");
	expect(elements.get("#setup-banner").classList.contains("hidden")).toBe(false);
	expect(elements.get("#send-button").disabled).toBe(true);
	expect(elements.get("#message-input").disabled).toBe(true);
});
