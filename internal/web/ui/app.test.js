import { expect, mock, test } from "bun:test";
import { readFile } from "node:fs/promises";

mock.module("lucide", () => ({
	ArrowUp: {}, Bot: {}, ChevronDown: {}, Copy: {}, Info: {}, LoaderCircle: {}, Mic: {}, MessagesSquare: {}, NotebookPen: {}, Pencil: {}, Send: {}, Settings: {}, Sparkles: {}, UserRound: {}, X: {},
	createIcons() {},
}));

class Element {
	constructor() {
		this.children = [];
		this.listeners = {};
		this.attributes = {};
		this.classes = new Set();
		this.style = {};
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
	getAttribute(name) { return this.attributes[name] ?? null; }
	addEventListener(name, callback) { this.listeners[name] = callback; }
	focus() { this.focused = true; }
	scrollIntoView(options) { this.scrollOptions = options; }
	get options() { return this.children; }
}

function renderedText(element) {
	return [element.textContent || "", element.innerHTML || "", ...(element.children || []).map(renderedText)].join(" ");
}

function findByAttribute(element, name) {
	if (element.attributes?.[name] !== undefined) return element;
	for (const child of element.children || []) {
		const match = findByAttribute(child, name);
		if (match) return match;
	}
	return null;
}

test("submission clears the input and formats the pending bubble before the reply", async () => {
	const elements = new Map();
	const windowListeners = {};
	const documentListeners = {};
	const memoryTabs = ["daily", "weekly", "monthly"].map((tab) => {
		const button = new Element();
		button.dataset = { memoryTab: tab };
		return button;
	});
	globalThis.document = {
		visibilityState: "visible",
		querySelector(selector) {
			if (!elements.has(selector)) elements.set(selector, new Element());
			return elements.get(selector);
		},
		querySelectorAll(selector) { return selector === "[data-memory-tab]" ? memoryTabs : []; },
		addEventListener(name, callback) { documentListeners[name] = callback; },
		createElement: () => new Element(),
		createTextNode: (text) => ({ textContent: text }),
	};
	const sessionValues = new Map();
	let profileSyncCallback;
	let profileSyncInterval = 0;
	let syncTimeout;
	let syncTimeoutMs = 0;
	globalThis.window = {
		sessionStorage: { getItem: (key) => sessionValues.get(key) ?? null, setItem: (key, value) => sessionValues.set(key, value), removeItem: (key) => sessionValues.delete(key) },
		matchMedia: () => ({ matches: true }),
		addEventListener(name, callback) { windowListeners[name] = callback; },
		setInterval(callback, interval) { profileSyncCallback = callback; profileSyncInterval = interval; return 1; },
		clearInterval() {},
		setTimeout(callback, ms) { syncTimeout = callback; syncTimeoutMs = ms; return 1; },
		clearTimeout() {},
	};
	let finishSend;
	let rejectSend;
	const messageRequests = [];
	const settingsSaves = [];
	const modelRequests = [];
	let activeState = { setup_required: false, profile: { slug: "artist", name: "Artist" }, model: "model-a", revision: 1, pending_notes: 0 };
	let delayedArtistModels = false;
	let resolveArtistModels;
	let delayedModelsProfile = "";
	let resolveDeferredModels;
	let failedModelsProfile = "";
	let unavailableModelsProfile = "";
	let failSave = false;
	let saveErrorCode = "settings_save_failed";
	let stateGets = 0;
	let deferNextStateResponse = false;
	let resolveDeferredState;
	let hangNextState = false;
	let delayedMemoryProfile = "";
	let resolveDelayedMemory;
	let failedMemoryTab = "";
	let deferOlderMemory = false;
	let resolveOlderMemory;
	let delayedHistoryProfile = "";
	let resolveDelayedHistory;
	let historyLoads = 0;
	let memoryLoads = 0;
	const historyProfiles = [];
	const memoryProfiles = [];
	const memoryTabRequests = [];
	globalThis.fetch = async (url, options = {}) => {
		if (url === "/api/messages") {
			messageRequests.push(JSON.parse(options.body));
			return new Promise((resolve, reject) => { finishSend = resolve; rejectSend = reject; });
		}
		if (url.startsWith("/api/messages?")) {
			historyLoads++;
			const profile = activeState.profile.slug;
			historyProfiles.push(profile);
			if (profile === delayedHistoryProfile && url.includes("before=")) return new Promise((resolve) => { resolveDelayedHistory = resolve; });
			const items = profile === "beta" ? [{ id: "beta-history", role: "user", content: "beta history", content_html: "<p>beta history</p>", created_at: "2026-11-20T00:00:00Z", conversation_id: "beta-conv" }] : [];
			return { ok: true, json: async () => ({ items, next_cursor: ["artist", "gamma"].includes(profile) ? `${profile}-cursor` : null }) };
		}
		if (url.startsWith("/api/memory?")) {
			memoryLoads++;
			const profile = activeState.profile.slug;
			memoryProfiles.push(profile);
			const query = new URLSearchParams(url.split("?")[1]);
			const tab = query.get("tab");
			const offset = query.get("offset");
			memoryTabRequests.push({ profile, tab, offset });
			if (profile === delayedMemoryProfile) return new Promise((resolve) => { resolveDelayedMemory = resolve; });
			if (failedMemoryTab === tab) return { ok: false, status: 500, json: async () => ({ error: { message: "memory unavailable" } }) };
			if (profile === "artist" && tab === "weekly" && offset === "1" && deferOlderMemory) {
				return new Promise((resolve) => { resolveOlderMemory = resolve; });
			}
			if (profile === "artist" && tab === "weekly") {
				const olderPage = offset === "1";
				const items = [{ id: olderPage ? "weekly-older" : "weekly-newer", content: olderPage ? "weekly older note" : "weekly newer note", content_html: `<p>${olderPage ? "weekly older note" : "weekly newer note"}</p>`, period_start: olderPage ? "2026-11-16" : "2026-11-22", period_key: "week" }];
				return { ok: true, json: async () => ({ items, next_offset: olderPage ? null : 1 }) };
			}
			const items = profile === "beta" ? [{ id: "beta-memory", content: "beta memory", content_html: "<p>beta memory</p>", created_at: "2026-11-20T00:00:00Z", freshness_state: "stale" }] : [];
			return { ok: true, json: async () => ({ items, next_offset: null }) };
		}
		if (url === "/api/state") {
			stateGets++;
			if (hangNextState) {
				hangNextState = false;
				return new Promise((_, reject) => { options.signal.addEventListener("abort", () => reject(new DOMException("Aborted", "AbortError")), { once: true }); });
			}
			if (deferNextStateResponse) {
				deferNextStateResponse = false;
				const snapshot = { ...activeState, profile: { ...activeState.profile } };
				return new Promise((resolve) => { resolveDeferredState = () => resolve({ ok: true, json: async () => snapshot }); });
			}
			return { ok: true, json: async () => activeState };
		}
		if (url === "/api/settings" && options.method === "POST") {
			const request = JSON.parse(options.body);
			settingsSaves.push(request);
			if (failSave) return { ok: false, status: saveErrorCode === "settings_changed" ? 409 : saveErrorCode === "settings_save_failed" ? 500 : 503, json: async () => ({ error: { message: "settings failed", code: saveErrorCode } }) };
			const profileName = request.profile_slug === "beta" ? "Beta" : request.profile_slug === "artist" ? "Artist" : "Gamma";
			activeState = { ...activeState, profile: { slug: request.profile_slug, name: profileName }, model: request.model, setup_required: request.profile_slug === "gamma", revision: activeState.revision + 1 };
			return { ok: true, json: async () => activeState };
		}
		if (url === "/api/settings") return { ok: true, json: async () => ({ active_profile_slug: activeState.profile.slug, profiles: [{ slug: "artist", name: "Artist" }, { slug: "beta", name: "Beta" }, { slug: "gamma", name: "Gamma" }] }) };
		if (url.startsWith("/api/settings/models?")) {
			const profile = new URLSearchParams(url.split("?")[1]).get("profile");
			modelRequests.push(profile);
			if (failedModelsProfile === profile) return { ok: false, status: 502, json: async () => ({ error: { message: "model list failed", code: "models_unavailable" } }) };
			if (profile === delayedModelsProfile) return new Promise((resolve) => { resolveDeferredModels = resolve; });
			if (profile === "artist" && delayedArtistModels) {
				return new Promise((resolve) => { resolveArtistModels = resolve; });
			}
			const savedModel = profile === activeState.profile.slug ? activeState.model : profile === "artist" ? "model-a" : "beta-model-1";
			const body = profile === "gamma"
				? { profile_slug: profile, saved_model: "", models: [], saved_embeddings_model: "", embedding_models: [], saved_extractor_model: "", setup_required: true }
				: {
					profile_slug: profile,
					saved_model: savedModel,
					models: profile === unavailableModelsProfile ? [] : profile === "artist" ? ["model-a"] : ["beta-model-1", "beta-model-2"],
					models_unavailable: profile === unavailableModelsProfile,
					saved_embeddings_model: profile === "artist" ? "embed-a" : "embed-b",
					embedding_models: ["embed-a", "embed-b"],
					saved_extractor_model: profile === "artist" ? "extractor-a" : "",
					setup_required: false,
				};
			return { ok: true, json: async () => body };
		}
		const body = url === "/api/markdown" ? { content_html: "<p>First</p>\n<p><strong>Second</strong></p>\n" } : { items: [], next_cursor: null, next_offset: null };
		return { ok: true, json: async () => body };
	};
	await import("./app.js");
	await Bun.sleep(0);
	expect(memoryTabs[0].attributes["aria-selected"]).toBe("true");
	expect(elements.get("#memory-panel-content").attributes["aria-labelledby"]).toBe("memory-tab-daily");
	expect(renderedText(elements.get("#memory-list"))).toContain("No notes captured yet.");
	failedMemoryTab = "weekly";
	memoryTabs[1].listeners.click();
	expect(renderedText(elements.get("#memory-status"))).toContain("Loading memory");
	await Bun.sleep(0);
	expect(renderedText(elements.get("#memory-status"))).toContain("Could not load memory: memory unavailable");
	failedMemoryTab = "";
	memoryTabs[1].listeners.click();
	await Bun.sleep(0);
	expect(memoryTabs[1].attributes["aria-selected"]).toBe("true");
	expect(memoryTabRequests.at(-1)).toMatchObject({ tab: "weekly", offset: "0" });
	expect(renderedText(elements.get("#memory-list"))).toContain("weekly newer note");
	const memoryScroll = elements.get("#memory-scroll");
	memoryScroll.scrollTop = 40;
	memoryScroll.scrollHeight = 120;
	deferOlderMemory = true;
	const loadOlderMemory = elements.get("#memory-list").children.find((child) => child.textContent === "Load older memory");
	loadOlderMemory.listeners.click();
	await Bun.sleep(0);
	expect(typeof resolveOlderMemory).toBe("function");
	memoryScroll.scrollHeight = 260;
	resolveOlderMemory({ ok: true, json: async () => ({ items: [{ id: "weekly-older", content: "weekly older note", content_html: "<p>weekly older note</p>", period_start: "2026-11-16", period_key: "week" }], next_offset: null }) });
	await Bun.sleep(0);
	const weeklyContent = renderedText(elements.get("#memory-list"));
	expect(weeklyContent.indexOf("weekly older note")).toBeLessThan(weeklyContent.indexOf("weekly newer note"));
	expect(memoryScroll.scrollTop).toBe(180);
	deferOlderMemory = false;
	memoryTabs[2].listeners.click();
	await Bun.sleep(0);
	expect(memoryTabs[2].attributes["aria-selected"]).toBe("true");
	expect(renderedText(elements.get("#memory-list"))).toContain("No summaries available yet.");
	memoryTabs[0].listeners.click();
	await Bun.sleep(0);
	const input = elements.get("#message-input");
	input.value = "First\n\n**Second**";
	elements.get("#message-form").listeners.submit({ preventDefault() {} });
	expect(input.value).toBe("");
	expect(messageRequests[0]?.profile_slug).toBe("artist");
	let row = elements.get("#message-list").children.find((child) => child.attributes?.["aria-label"] === "Your message");
	let body = findByAttribute(row, "data-message-content");
	expect(body.classes.has("whitespace-pre-wrap")).toBe(true);
	expect(body.textContent).toBe("First\n\n**Second**");
	await Bun.sleep(0);
	row = elements.get("#message-list").children.find((child) => child.attributes?.["aria-label"] === "Your message");
	body = findByAttribute(row, "data-message-content");
	expect(body.innerHTML).toBe("<p>First</p>\n<p><strong>Second</strong></p>\n");
	input.value = "new draft";
	rejectSend(new TypeError("Failed to fetch"));
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

	input.value = "retry this message";
	elements.get("#message-form").listeners.submit({ preventDefault() {} });
	const retryRequestID = messageRequests.at(-1).request_id;
	rejectSend(new TypeError("Failed to fetch"));
	await Bun.sleep(0);
	expect(input.value).toBe("retry this message");
	elements.get("#message-form").listeners.submit({ preventDefault() {} });
	expect(messageRequests.at(-1).request_id).toBe(retryRequestID);
	finishSend({ ok: true, json: async () => ({}) });
	await Bun.sleep(0);
	expect(Boolean(elements.get("#settings-button")?.listeners.click)).toBe(true);

	const settingsButton = elements.get("#settings-button");
	input.value = "unfinished turn";
	elements.get("#message-form").listeners.submit({ preventDefault() {} });
	await Bun.sleep(0);
	await settingsButton.listeners.click();
	expect(elements.get("#settings-profile").value).toBe("artist");
	expect(elements.get("#settings-model").value).toBe("model-a");
	for (const id of ["settings-profile-search", "settings-model-search", "settings-embeddings-search", "settings-extractor-search"]) {
		expect(elements.get(`#${id}`).attributes.role).toBe("combobox");
	}
	const embeddingInfoButton = elements.get("#settings-embeddings-info-button");
	expect(embeddingInfoButton.attributes["aria-label"]).toContain("embedding");
	embeddingInfoButton.listeners.click();
	expect(elements.get("#settings-embeddings-info").classList.contains("hidden")).toBe(false);
	expect(embeddingInfoButton.attributes["aria-expanded"]).toBe("true");
	embeddingInfoButton.listeners.click();
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
	const profileSearch = elements.get("#settings-profile-search");
	profileSearch.value = "no-such-profile";
	profileSearch.listeners.input();
	expect(renderedText(elements.get("#settings-profile-options"))).toContain("No matching options");
	expect(elements.get("#settings-profile-options").children[0].attributes.role).toBe("option");
	expect(elements.get("#settings-profile-options").children[0].attributes["aria-disabled"]).toBe("true");
	profileSearch.listeners.blur();
	expect(profileSearch.value).toBe("Artist");
	expect(elements.get("#settings-profile-options").classList.contains("hidden")).toBe(true);
	profileSearch.value = "no-such-profile";
	profileSearch.listeners.input();
	let stoppedOpenEscape = false;
	profileSearch.listeners.keydown({ key: "Escape", preventDefault() {}, stopPropagation() { stoppedOpenEscape = true; } });
	expect(profileSearch.attributes["aria-expanded"]).toBe("false");
	expect(stoppedOpenEscape).toBe(true);
	let stoppedClosedEscape = false;
	profileSearch.listeners.keydown({ key: "Escape", preventDefault() {}, stopPropagation() { stoppedClosedEscape = true; } });
	expect(stoppedClosedEscape).toBe(false);
	profileSearch.value = "bet";
	profileSearch.listeners.input();
	expect(renderedText(elements.get("#settings-profile-options"))).toContain("Beta");
	const betaOption = elements.get("#settings-profile-options").children[0];
	expect(betaOption.attributes.tabindex).toBe("-1");
	let preventedMouseFocus = false;
	betaOption.listeners.mousedown({ preventDefault() { preventedMouseFocus = true; } });
	expect(preventedMouseFocus).toBe(true);
	profileSearch.listeners.keydown({ key: "ArrowDown", preventDefault() {} });
	expect(profileSearch.attributes["aria-activedescendant"]).toContain("settings-profile-options-option-");
	expect(elements.get("#settings-profile-options").children[0].classList.contains("bg-raised")).toBe(true);
	profileSearch.listeners.keydown({ key: "Enter", preventDefault() {} });
	expect(profileSelect.value).toBe("beta");
	await Bun.sleep(0);
	profileSearch.listeners.click();
	expect(elements.get("#settings-profile-options").classList.contains("hidden")).toBe(false);
	profileSearch.listeners.keydown({ key: "Escape", preventDefault() {} });
	expect(elements.get("#settings-profile-options").classList.contains("hidden")).toBe(true);
	expect(profileSelect.disabled).toBe(false);
	const modelSelect = elements.get("#settings-model");
	const embeddingSelect = elements.get("#settings-embeddings");
	const extractorSelect = elements.get("#settings-extractor");
	expect(modelSelect.value).toBe("beta-model-1");
	expect(embeddingSelect.value).toBe("embed-b");
	expect(extractorSelect.value).toBe("");
	const modelSearch = elements.get("#settings-model-search");
	modelSearch.value = "model-2";
	modelSearch.listeners.input();
	expect(renderedText(elements.get("#settings-model-options"))).toContain("beta-model-2");
	let preventedListboxBlur = false;
	elements.get("#settings-model-options").listeners.mousedown({ preventDefault() { preventedListboxBlur = true; } });
	expect(preventedListboxBlur).toBe(true);
	modelSearch.listeners.blur();
	expect(modelSelect.value).toBe("beta-model-1");
	expect(modelSearch.value).toBe("beta-model-1");
	expect(elements.get("#settings-model-options").classList.contains("hidden")).toBe(true);
	modelSearch.value = "model-2";
	modelSearch.listeners.input();
	modelSearch.listeners.keydown({ key: "ArrowDown", preventDefault() {} });
	expect(elements.get("#settings-model-options").children[0].scrollOptions).toEqual({ block: "nearest" });
	modelSearch.listeners.keydown({ key: "Enter", preventDefault() {} });
	expect(modelSelect.value).toBe("beta-model-2");
	resolveArtistModels({ ok: true, json: async () => ({ profile_slug: "artist", saved_model: "model-a", models: ["model-a"], setup_required: false }) });
	await opening;
	expect(modelSelect.value).toBe("beta-model-2");
	profileSelect.value = "beta";
	elements.get("#settings-cancel").listeners.click();
	expect(settingsSaves.length).toBe(0);
	expect(activeState.profile.slug).toBe("artist");
	expect(input.value).toBe("draft preserved by cancel");
	expect(elements.get("#settings-panel").classList.contains("hidden")).toBe(true);
	expect(settingsButton.focused).toBe(true);

	delayedArtistModels = false;
	delayedModelsProfile = "artist";
	const reopening = settingsButton.listeners.click();
	await Bun.sleep(0);
	await Bun.sleep(0);
	expect(typeof resolveDeferredModels).toBe("function");
	const betaModelBeforeClose = modelSelect.value;
	elements.get("#settings-cancel").listeners.click();
	resolveDeferredModels({ ok: true, json: async () => ({ profile_slug: "artist", saved_model: "model-a", models: ["model-a"], setup_required: false }) });
	await reopening;
	await Bun.sleep(0);
	expect(elements.get("#settings-panel").classList.contains("hidden")).toBe(true);
	expect(modelSelect.value).toBe(betaModelBeforeClose);
	delayedModelsProfile = "";
	await settingsButton.listeners.click();
	expect(modelSelect.value).toBe("model-a");
	elements.get("#settings-cancel").listeners.click();

	await settingsButton.listeners.click();
	profileSelect.value = "beta";
	await profileSelect.listeners.change();
	modelSearch.value = "model-2";
	modelSearch.listeners.input();
	modelSearch.listeners.keydown({ key: "ArrowDown", preventDefault() {} });
	modelSearch.listeners.keydown({ key: "Enter", preventDefault() {} });
	const embeddingSearch = elements.get("#settings-embeddings-search");
	embeddingSearch.value = "disabled";
	embeddingSearch.listeners.input();
	embeddingSearch.listeners.keydown({ key: "ArrowDown", preventDefault() {} });
	embeddingSearch.listeners.keydown({ key: "Enter", preventDefault() {} });
	expect(embeddingSelect.value).toBe("");
	expect(elements.get("#settings-embedding-warning").classList.contains("hidden")).toBe(true);
	expect(elements.get("#settings-save").disabled).toBe(false);
	embeddingSearch.value = "embed-a";
	embeddingSearch.listeners.input();
	expect(renderedText(elements.get("#settings-embeddings-options"))).toContain("embed-a");
	embeddingSearch.listeners.keydown({ key: "ArrowDown", preventDefault() {} });
	embeddingSearch.listeners.keydown({ key: "Enter", preventDefault() {} });
	const extractorSearch = elements.get("#settings-extractor-search");
	extractorSearch.value = "fallback";
	extractorSearch.listeners.input();
	extractorSearch.listeners.keydown({ key: "ArrowDown", preventDefault() {} });
	extractorSearch.listeners.keydown({ key: "Enter", preventDefault() {} });
	expect(elements.get("#settings-embedding-warning").classList.contains("hidden")).toBe(false);
	expect(elements.get("#settings-save").disabled).toBe(true);
	const rebuildConfirm = elements.get("#settings-confirm-embedding-rebuild");
	rebuildConfirm.checked = true;
	rebuildConfirm.listeners.change();
	const oldHistoryLoads = historyLoads;
	const oldMemoryLoads = memoryLoads;
	const sentMessagesBeforeSave = messageRequests.length;
	input.value = "unsent prompt to preserve";
	await elements.get("#settings-form").listeners.submit({ preventDefault() {} });
	expect(settingsSaves).toHaveLength(1);
	expect(settingsSaves[0]).toEqual({ profile_slug: "beta", model: "beta-model-2", embeddings_model: "embed-a", extractor_model: "", revision: 1 });
	expect(activeState.profile.slug).toBe("beta");
	expect(activeState.model).toBe("beta-model-2");
	expect(input.value).toBe("unsent prompt to preserve");
	expect(messageRequests).toHaveLength(sentMessagesBeforeSave);
	expect(historyLoads).toBeGreaterThan(oldHistoryLoads);
	expect(memoryLoads).toBeGreaterThan(oldMemoryLoads);
	expect(elements.get("#settings-panel").classList.contains("hidden")).toBe(true);
	expect(elements.get("#profile-name").textContent).toBe("Beta");

	unavailableModelsProfile = "beta";
	await settingsButton.listeners.click();
	expect(modelSelect.value).toBe("beta-model-2");
	expect(modelSearch.value).toContain("saved, unavailable");
	expect(elements.get("#settings-model-help").textContent).toContain("Saved chat and extractor choices are kept");
	expect(elements.get("#settings-retry-models").classList.contains("hidden")).toBe(false);
	elements.get("#settings-cancel").listeners.click();
	unavailableModelsProfile = "";

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
	expect(settingsSaves.at(-1)).toEqual({ profile_slug: "gamma", model: "", embeddings_model: "", extractor_model: "", revision: 4 });
	expect(activeState.setup_required).toBe(true);
	expect(elements.get("#settings-panel").classList.contains("hidden")).toBe(true);

	expect(typeof windowListeners.focus).toBe("function");
	expect(typeof documentListeners.visibilitychange).toBe("function");
	expect(profileSyncInterval).toBeGreaterThan(0);
	input.value = "cross-tab unsent draft";
	activeState = { ...activeState, setup_required: false, profile: { slug: "artist", name: "Artist" }, model: "model-a", revision: activeState.revision + 1, pending_notes: 0 };
	await windowListeners.focus();
	expect(elements.get("#profile-name").textContent).toBe("Artist");
	delayedHistoryProfile = "artist";
	const olderButton = elements.get("#message-list").children.find((child) => child.innerHTML?.includes("Load older messages"));
	expect(olderButton).toBeDefined();
	olderButton.listeners.click();
	delayedMemoryProfile = "artist";
	memoryTabs[0].listeners.click();
	await Bun.sleep(0);
	expect(typeof resolveDelayedHistory).toBe("function");
	expect(typeof resolveDelayedMemory).toBe("function");
	const crossTabMessagePosts = messageRequests.length;
	elements.get("#chat-status").classList.remove("hidden");
	elements.get("#chat-status").replaceChildren(Object.assign(new Element(), { textContent: "stale profile error" }));
	elements.get("#memory-status").classList.remove("hidden");
	elements.get("#memory-status").replaceChildren(Object.assign(new Element(), { textContent: "stale memory status" }));
	activeState = { setup_required: false, profile: { slug: "beta", name: "Beta" }, model: "beta-model-2", revision: 6, pending_notes: 0 };
	document.visibilityState = "hidden";
	const stateGetsWhileHidden = stateGets;
	await windowListeners.focus();
	await documentListeners.visibilitychange();
	await profileSyncCallback();
	expect(stateGets).toBe(stateGetsWhileHidden);
	expect(elements.get("#profile-name").textContent).toBe("Artist");
	document.visibilityState = "visible";
	await documentListeners.visibilitychange();
	expect(elements.get("#profile-name").textContent).toBe("Beta");
	expect(input.value).toBe("cross-tab unsent draft");
	expect(elements.get("#chat-status").classList.contains("hidden")).toBe(true);
	expect(elements.get("#memory-status").classList.contains("hidden")).toBe(true);
	expect(renderedText(elements.get("#message-list"))).toContain("beta history");
	expect(renderedText(elements.get("#memory-list"))).toContain("beta memory");
	expect(renderedText(elements.get("#memory-list"))).toContain("stale");
	expect(messageRequests).toHaveLength(crossTabMessagePosts);
	expect(historyProfiles.at(-1)).toBe("beta");
	expect(memoryProfiles.at(-1)).toBe("beta");
	activeState = { ...activeState, model: "beta-model-1", revision: 7 };
	await profileSyncCallback();
	expect(elements.get("#model-name").textContent).toBe("beta-model-1");
	activeState = { ...activeState, model: "beta-model-2", revision: 8 };
	await documentListeners.visibilitychange();
	expect(elements.get("#model-name").textContent).toBe("beta-model-2");
	resolveDelayedHistory({ ok: true, json: async () => ({ items: [{ id: "stale-artist", role: "user", content: "stale artist history", content_html: "<p>stale artist history</p>", created_at: "2026-11-19T00:00:00Z", conversation_id: "artist-conv" }], next_cursor: null }) });
	resolveDelayedMemory({ ok: true, json: async () => ({ items: [{ id: "stale-artist-note", content: "stale artist memory", content_html: "<p>stale artist memory</p>", created_at: "2026-11-19T00:00:00Z" }], next_offset: null }) });
	await Bun.sleep(0);
	expect(renderedText(elements.get("#message-list"))).not.toContain("stale artist history");
	expect(renderedText(elements.get("#memory-list"))).not.toContain("stale artist memory");
	delayedHistoryProfile = "";
	delayedMemoryProfile = "";

	await settingsButton.listeners.click();
	profileSelect.value = "artist";
	await profileSelect.listeners.change();
	activeState = { setup_required: true, profile: { slug: "gamma", name: "Gamma" }, model: "", revision: 9, pending_notes: 0 };
	await windowListeners.focus();
	expect(elements.get("#profile-name").textContent).toBe("Gamma");
	expect(elements.get("#settings-save").disabled).toBe(true);
	expect(elements.get("#settings-status").children[0]?.textContent).toContain("changed in another tab");
	const settingsReload = elements.get("#settings-status").children.at(-1);
	await settingsReload.listeners.click();
	expect(profileSelect.value).toBe("gamma");
	expect(modelSelect.options).toHaveLength(0);
	expect(elements.get("#settings-save").disabled).toBe(false);
	elements.get("#settings-cancel").listeners.click();

	activeState = { setup_required: false, profile: { slug: "beta", name: "Beta" }, model: "beta-model-2", revision: 10, pending_notes: 0 };
	await windowListeners.focus();
	expect(elements.get("#profile-name").textContent).toBe("Beta");
	deferNextStateResponse = true;
	const stalePoll = windowListeners.focus();
	await Bun.sleep(0);
	expect(typeof resolveDeferredState).toBe("function");
	await settingsButton.listeners.click();
	profileSelect.value = "artist";
	await profileSelect.listeners.change();
	await elements.get("#settings-form").listeners.submit({ preventDefault() {} });
	expect(activeState.profile.slug).toBe("artist");
	expect(elements.get("#profile-name").textContent).toBe("Artist");
	resolveDeferredState();
	await stalePoll;
	expect(elements.get("#profile-name").textContent).toBe("Artist");
	activeState = { ...activeState, profile: { slug: "beta", name: "Beta" }, model: "beta-model-2", revision: 12, setup_required: false };
	await windowListeners.focus();
	expect(elements.get("#profile-name").textContent).toBe("Beta");
	hangNextState = true;
	const hungSync = windowListeners.focus();
	await Bun.sleep(0);
	expect(syncTimeoutMs).toBe(5000);
	const stateGetsWhileHung = stateGets;
	await windowListeners.focus();
	expect(stateGets).toBe(stateGetsWhileHung);
	syncTimeout();
	await hungSync;
	activeState = { ...activeState, revision: 13 };
	await windowListeners.focus();
	expect(elements.get("#profile-name").textContent).toBe("Beta");

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

test("settings explanation describes incompatible embedding spaces and index rebuild costs", async () => {
	const html = await readFile(new URL("./index.html", import.meta.url), "utf8");
	for (const phrase of [
		"The index stores numerical representations—embeddings—of your memories.",
		"That comparison only makes sense when the question and memories were embedded using compatible models.",
		"Think of it like coordinates on a map",
		"Generate new embeddings for each memory using the new model.",
		"generating embeddings can take time and may cost money",
		"it’s safest not to assume that.",
	]) {
		expect(html).toContain(phrase);
	}
});
