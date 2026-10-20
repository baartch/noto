import {
	ArrowUp,
	Bot,
	ChevronDown,
	LoaderCircle,
	MessagesSquare,
	NotebookPen,
	Sparkles,
	createIcons,
} from "lucide";
import { clearSubmittedDraft, createPendingUserMessage, loadSendAttempt, mergeOlderMessages, pollUntil, saveSendAttempt, scrollToBottom, sendWithAttempt, sortMemoryOldestFirst } from "./state.js";

const icons = { ArrowUp, Bot, ChevronDown, LoaderCircle, MessagesSquare, NotebookPen, Sparkles };
const $ = (selector) => document.querySelector(selector);
const ui = {
	profile: $("#profile-name"),
	model: $("#model-name"),
	setupBanner: $("#setup-banner"),
	chatStatus: $("#chat-status"),
	chatPanel: $("#chat-panel"),
	memoryPanel: $("#memory-panel"),
	messageScroll: $("#message-scroll"),
	messageList: $("#message-list"),
	messageForm: $("#message-form"),
	messageInput: $("#message-input"),
	sendButton: $("#send-button"),
	sendLabel: $("#send-label"),
	sendState: $("#send-state"),
	memoryList: $("#memory-list"),
	memoryStatus: $("#memory-status"),
	memoryScroll: $("#memory-scroll"),
	chatTab: $("#mobile-chat-tab"),
	mobileMemoryTab: $("#mobile-memory-tab"),
};

const state = {
	profile: "",
	model: "",
	setupRequired: true,
	sending: false,
	pendingNotes: 0,
	messages: [],
	pendingUserMessage: null,
	nextCursor: null,
	loadingOlder: false,
	memoryTab: "daily",
	memoryOffset: 0,
	memoryNextOffset: null,
	memoryItems: [],
	memoryRequest: null,
	memoryPoll: null,
	statePoll: null,
	sendAttempt: null,
	mobileView: "chat",
};
let attemptStorage = null;
try {
	attemptStorage = window.sessionStorage;
	state.sendAttempt = loadSendAttempt(attemptStorage);
	if (state.sendAttempt) ui.messageInput.value = state.sendAttempt.content;
} catch {
	// Browser storage can be disabled; keep retries safe for the current page.
}

class APIError extends Error {
	constructor(message, status, code = "") {
		super(message);
		this.name = "APIError";
		this.status = status;
		this.code = code;
	}
}

async function api(path, options = {}) {
	const response = await fetch(path, { credentials: "same-origin", ...options });
	const body = await response.json().catch(() => null);
	if (!response.ok) {
		throw new APIError(body?.error?.message || `Request failed (${response.status})`, response.status, body?.error?.code);
	}
	return body;
}


function showStatus(target, message, tone = "muted", action = null) {
	target.replaceChildren();
	if (message) {
		const text = document.createElement("span");
		text.textContent = message;
		target.append(text);
	}
	if (action) {
		const button = document.createElement("button");
		button.type = "button";
		button.className = "shrink-0 rounded-lg border border-line px-2.5 py-1.5 text-xs font-medium text-ink hover:bg-raised focus-visible:ring-2 focus-visible:ring-focus";
		button.textContent = action.label;
		button.addEventListener("click", action.run);
		target.append(button);
	}
	target.classList.toggle("hidden", !message);
	target.classList.toggle("text-danger", tone === "error");
	target.classList.toggle("text-muted", tone !== "error");
	target.classList.toggle("flex", Boolean(action));
	target.classList.toggle("items-center", Boolean(action));
	target.classList.toggle("justify-between", Boolean(action));
	target.classList.toggle("gap-3", Boolean(action));
}

function renderState(serverState) {
	state.profile = serverState.profile?.name || "Default profile";
	state.model = serverState.model || "Provider not configured";
	state.setupRequired = Boolean(serverState.setup_required);
	state.sending = Boolean(serverState.sending);
	state.pendingNotes = Number(serverState.pending_notes || 0);
	ui.profile.textContent = state.profile;
	ui.model.textContent = state.model;
	ui.setupBanner.classList.toggle("hidden", !state.setupRequired);
	updateSendControls();
}

function updateSendControls() {
	const disabled = state.setupRequired || state.sending;
	ui.sendButton.disabled = disabled;
	ui.messageInput.disabled = state.setupRequired;
	ui.sendLabel.textContent = state.sending ? "Thinking…" : "Send";
	ui.sendState.textContent = state.sending ? "Noto is working" : state.pendingNotes > 0 ? "Saving memory…" : "";
}

function formatDate(value, options = { weekday: "short", day: "2-digit", month: "short", year: "numeric" }) {
	const date = new Date(value);
	return Number.isNaN(date.valueOf()) ? "Unknown date" : new Intl.DateTimeFormat(undefined, options).format(date);
}

function renderMessages({ scrollToBottom: forceScrollToBottom = false } = {}) {
	const shouldStick = forceScrollToBottom || ui.messageScroll.scrollHeight - ui.messageScroll.scrollTop - ui.messageScroll.clientHeight < 80;
	ui.messageList.replaceChildren();
	if (state.nextCursor) {
		const olderButton = document.createElement("button");
		olderButton.type = "button";
		olderButton.className = "mx-auto inline-flex items-center gap-2 rounded-full border border-line bg-raised px-3 py-2 text-xs text-muted hover:text-ink focus-visible:ring-2 focus-visible:ring-focus";
		olderButton.innerHTML = '<i data-lucide="chevron-down" class="size-3.5" aria-hidden="true"></i>Load older messages';
		olderButton.addEventListener("click", loadOlderMessages);
		ui.messageList.append(olderButton);
	}
	const visibleMessages = state.pendingUserMessage ? [...state.messages, state.pendingUserMessage] : state.messages;
	if (visibleMessages.length === 0) {
		const empty = document.createElement("div");
		empty.className = "m-auto max-w-sm py-16 text-center";
		empty.innerHTML = '<div class="mx-auto mb-4 grid size-12 place-items-center rounded-2xl border border-line bg-raised text-accent"><i data-lucide="sparkles" class="size-5" aria-hidden="true"></i></div><h2 class="font-medium">Start with what’s on your mind</h2><p class="mt-2 text-sm text-muted">Noto will use your saved memory to keep the conversation connected.</p>';
		ui.messageList.append(empty);
	}
	let previousDate = "";
	let previousConversation = "";
	for (const message of visibleMessages) {
		const dateKey = new Date(message.created_at).toLocaleDateString();
		if (dateKey !== previousDate || (message.conversation_id && message.conversation_id !== previousConversation)) {
			const separator = document.createElement("div");
			separator.className = "flex items-center gap-3 py-1 text-[11px] text-muted";
			separator.innerHTML = '<span class="h-px flex-1 bg-line"></span><span class="font-mono">' + escapeHTML(formatDate(message.created_at)) + '</span><span class="h-px flex-1 bg-line"></span>';
			ui.messageList.append(separator);
			previousDate = dateKey;
			previousConversation = message.conversation_id || previousConversation;
		}
		ui.messageList.append(messageElement(message));
	}
	if (state.sending) ui.messageList.append(thinkingElement());
	createIcons({ icons });
	if (shouldStick) scrollToBottom(ui.messageScroll);
}

function messageElement(message) {
	const isUser = message.role === "user";
	const row = document.createElement("article");
	row.className = `flex ${isUser ? "justify-end" : "justify-start"}`;
	row.setAttribute("aria-label", isUser ? "Your message" : "Noto response");
	const bubble = document.createElement("div");
	bubble.className = `max-w-[92%] rounded-2xl border px-4 py-3 sm:max-w-[82%] ${isUser ? "border-accent/30 bg-active" : "border-line bg-panel"}`;
	const meta = document.createElement("div");
	meta.className = "mb-2 flex items-center justify-start gap-2 font-mono text-[10px] uppercase tracking-wide text-muted";
	const name = document.createElement("span");
	name.textContent = isUser ? "You" : "Noto";
	const timestamp = document.createElement("time");
	timestamp.dateTime = message.created_at || "";
	timestamp.textContent = formatDate(message.created_at, { hour: "2-digit", minute: "2-digit" });
	meta.append(name, document.createTextNode("·"), timestamp);
	const body = document.createElement("div");
	body.className = "markdown-body break-words text-sm leading-6 text-ink";
	if (message.optimistic) body.textContent = message.content;
	else body.innerHTML = message.content_html || "";
	bubble.append(meta, body);
	row.append(bubble);
	return row;
}

function thinkingElement() {
	const row = document.createElement("div");
	row.className = "inline-flex items-center gap-2 self-start rounded-xl border border-line bg-panel px-4 py-3 text-sm text-muted";
	row.setAttribute("role", "status");
	row.innerHTML = '<i data-lucide="loader-circle" class="size-4 animate-spin text-accent" aria-hidden="true"></i>Noto is thinking…';
	return row;
}

function escapeHTML(value) {
	return String(value).replace(/[&<>"']/g, (character) => ({
		"&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
	})[character]);
}

async function loadMessages({ older = false } = {}) {
	if (state.loadingOlder || (older && !state.nextCursor)) return;
	state.loadingOlder = older;
	const previousHeight = ui.messageScroll.scrollHeight;
	const previousTop = ui.messageScroll.scrollTop;
	try {
		const query = new URLSearchParams({ limit: "50" });
		if (older) query.set("before", state.nextCursor);
		const page = await api(`/api/messages?${query}`);
		state.messages = older ? mergeOlderMessages(state.messages, page.items) : page.items;
		state.nextCursor = page.next_cursor;
		renderMessages({ scrollToBottom: !older });
		if (older) ui.messageScroll.scrollTop = previousTop + (ui.messageScroll.scrollHeight - previousHeight);
	} catch (error) {
		showStatus(ui.chatStatus, `Could not load conversation: ${error.message}`, "error");
	} finally {
		state.loadingOlder = false;
	}
}

function loadOlderMessages() {
	void loadMessages({ older: true });
}

function renderMemory() {
	ui.memoryList.replaceChildren();
	if (state.memoryNextOffset !== null) {
		const more = document.createElement("button");
		more.type = "button";
		more.className = "mt-1 rounded-lg border border-line px-3 py-2 text-xs text-muted hover:text-ink focus-visible:ring-2 focus-visible:ring-focus";
		more.textContent = "Load older memory";
		more.addEventListener("click", loadOlderMemory);
		ui.memoryList.append(more);
	}
	if (state.memoryItems.length === 0) {
		const empty = document.createElement("p");
		empty.className = "rounded-xl border border-line bg-raised p-4 text-sm text-muted";
		empty.textContent = state.memoryTab === "daily" ? "No notes captured yet." : "No summaries available yet.";
		ui.memoryList.append(empty);
	}
	let lastDate = "";
	for (const item of state.memoryItems) {
		if (state.memoryTab === "daily") {
			const dateKey = new Date(item.created_at).toLocaleDateString();
			if (dateKey !== lastDate) {
				const heading = document.createElement("h3");
				heading.className = "px-1 pt-2 font-mono text-[10px] uppercase tracking-[0.14em] text-muted";
				heading.textContent = formatDate(item.created_at, { weekday: "short", day: "2-digit", month: "short" });
				ui.memoryList.append(heading);
				lastDate = dateKey;
			}
		}
		ui.memoryList.append(memoryCard(item));
	}
	createIcons({ icons });
}

function memoryCard(item) {
	const card = document.createElement("article");
	card.className = "rounded-xl border border-line bg-raised p-3.5 shadow-lg shadow-black/10";
	const meta = document.createElement("div");
	meta.className = "mb-2 flex items-center justify-between gap-2 font-mono text-[10px] uppercase tracking-wide text-muted";
	const left = document.createElement("span");
	if (state.memoryTab === "daily") {
		left.textContent = `${formatDate(item.created_at, { hour: "2-digit", minute: "2-digit" })} · ${item.category || "note"}`;
	} else {
		left.textContent = item.period_key || state.memoryTab;
	}
	meta.append(left);
	if (item.freshness_state && item.freshness_state !== "fresh") {
		const freshness = document.createElement("span");
		freshness.className = "text-warning";
		freshness.textContent = item.freshness_state;
		meta.append(freshness);
	}
	const body = document.createElement("div");
	body.className = "markdown-body break-words text-sm leading-5 text-ink";
	body.innerHTML = item.content_html || "";
	card.append(meta, body);
	return card;
}

async function loadMemory(tab = state.memoryTab, { offset = 0, append = false } = {}) {
	state.memoryRequest?.abort();
	const controller = new AbortController();
	state.memoryRequest = controller;
	state.memoryTab = tab;
	state.memoryOffset = offset;
	const previousHeight = ui.memoryScroll.scrollHeight;
	const previousTop = ui.memoryScroll.scrollTop;
	if (!append) showStatus(ui.memoryStatus, "Loading memory…");
	updateMemoryTabs();
	try {
		const query = new URLSearchParams({ tab, offset: String(offset), limit: "50" });
		const page = await api(`/api/memory?${query}`, { signal: controller.signal });
		if (controller.signal.aborted || state.memoryRequest !== controller) return;
		state.memoryItems = sortMemoryOldestFirst(append ? [...state.memoryItems, ...page.items] : page.items, tab);
		state.memoryNextOffset = page.next_offset;
		renderMemory();
		if (append) ui.memoryScroll.scrollTop = previousTop + (ui.memoryScroll.scrollHeight - previousHeight);
		else scrollToBottom(ui.memoryScroll);
		showStatus(ui.memoryStatus, "");
	} catch (error) {
		if (error.name !== "AbortError") showStatus(ui.memoryStatus, `Could not load memory: ${error.message}`, "error");
	} finally {
		if (state.memoryRequest === controller) state.memoryRequest = null;
	}
}

function loadOlderMemory() {
	if (state.memoryNextOffset === null) return;
	void loadMemory(state.memoryTab, { offset: state.memoryNextOffset, append: true });
}

function updateMemoryTabs() {
	for (const button of document.querySelectorAll("[data-memory-tab]")) {
		const selected = button.dataset.memoryTab === state.memoryTab;
		button.setAttribute("aria-selected", String(selected));
		button.classList.toggle("bg-active", selected);
		button.classList.toggle("text-ink", selected);
		button.classList.toggle("text-muted", !selected);
	}
}

async function refreshState() {
	const serverState = await api("/api/state");
	renderState(serverState);
	return serverState;
}

async function pollServerTurn() {
	state.statePoll?.abort();
	const controller = new AbortController();
	state.statePoll = controller;
	try {
		const settled = await pollUntil((signal) => api("/api/state", { signal }), (value) => !value.sending, {
			maxAttempts: 300,
			intervalMs: 1000,
			signal: controller.signal,
		});
		if (!controller.signal.aborted && settled) {
			renderState(settled);
			state.pendingUserMessage = null;
			await loadMessages();
			if (settled.pending_notes > 0) void pollMemoryExtraction();
			else await loadMemory(state.memoryTab);
		} else if (!controller.signal.aborted) {
			showStatus(ui.chatStatus, "The chat is still processing.", "muted", { label: "Check again", run: () => void pollServerTurn() });
		}
	} catch (error) {
		if (!controller.signal.aborted) showStatus(ui.chatStatus, `Could not check chat status: ${error.message}`, "error", { label: "Check again", run: () => void pollServerTurn() });
	} finally {
		if (state.statePoll === controller) state.statePoll = null;
	}
}

async function pollMemoryExtraction() {
	state.memoryPoll?.abort();
	const controller = new AbortController();
	state.memoryPoll = controller;
	try {
		const settled = await pollUntil((signal) => api("/api/state", { signal }), (value) => value.pending_notes === 0, {
			maxAttempts: 60,
			intervalMs: 1000,
			signal: controller.signal,
		});
		if (!controller.signal.aborted && settled) {
			renderState(settled);
			await loadMemory(state.memoryTab);
		} else if (!controller.signal.aborted) {
			showStatus(ui.memoryStatus, "Memory extraction is still running.", "muted", { label: "Refresh notes", run: () => void loadMemory(state.memoryTab) });
		}
	} catch (error) {
		if (!controller.signal.aborted) showStatus(ui.memoryStatus, `Could not refresh memory: ${error.message}`, "error", { label: "Refresh notes", run: () => void loadMemory(state.memoryTab) });
	} finally {
		if (state.memoryPoll === controller) state.memoryPoll = null;
	}
}

async function sendMessage() {
	if (state.setupRequired || state.sending) return;
	const content = ui.messageInput.value;
	if (!content.trim()) return;
	state.sending = true;
	showStatus(ui.chatStatus, "");
	updateSendControls();
	const result = await sendWithAttempt(content, state.sendAttempt, async (attempt) => {
		state.sendAttempt = attempt;
		saveSendAttempt(attemptStorage, attempt);
		state.pendingUserMessage = {
			...createPendingUserMessage(attempt),
			conversation_id: state.messages.at(-1)?.conversation_id || "",
		};
		renderMessages({ scrollToBottom: true });
		return api("/api/messages", {
			method: "POST",
			headers: { "Content-Type": "application/json" },
			body: JSON.stringify({ request_id: attempt.requestID, content: attempt.content }),
		});
	});
	state.sending = false;
	state.sendAttempt = result.attempt;
	saveSendAttempt(attemptStorage, result.attempt);
	if (result.error) {
		if (!result.attempt) state.pendingUserMessage = null;
		updateSendControls();
		showStatus(ui.chatStatus, `${result.error.message}. Your draft is kept; it was not sent again automatically.`, "error");
		try {
			const serverState = await refreshState();
			if (serverState.sending) void pollServerTurn();
			else {
				state.pendingUserMessage = null;
				await loadMessages();
			}
		} catch {
			// Keep the draft and request ID so a manual retry cannot duplicate an accepted turn.
		}
		renderMessages({ scrollToBottom: true });
		return;
	}
	state.pendingUserMessage = null;
	ui.messageInput.value = clearSubmittedDraft(ui.messageInput.value, content);
	state.sendAttempt = null;
	updateSendControls();
	await loadMessages();
	try {
		const serverState = await refreshState();
		if (serverState.pending_notes > 0) void pollMemoryExtraction();
		else await loadMemory(state.memoryTab);
	} catch (error) {
		showStatus(ui.chatStatus, `Reply received; status refresh failed: ${error.message}`, "error");
	}
}

function setMobileView(view) {
	state.mobileView = view;
	const desktop = window.matchMedia("(min-width: 1024px)").matches;
	ui.chatPanel.classList.toggle("hidden", !desktop && view !== "chat");
	ui.memoryPanel.classList.toggle("hidden", !desktop && view !== "memory");
	ui.chatTab.setAttribute("aria-pressed", String(view === "chat"));
	ui.mobileMemoryTab.setAttribute("aria-pressed", String(view === "memory"));
	ui.chatTab.classList.toggle("border-accent", view === "chat");
	ui.chatTab.classList.toggle("text-ink", view === "chat");
	ui.chatTab.classList.toggle("border-transparent", view !== "chat");
	ui.chatTab.classList.toggle("text-muted", view !== "chat");
	ui.mobileMemoryTab.classList.toggle("border-accent", view === "memory");
	ui.mobileMemoryTab.classList.toggle("text-ink", view === "memory");
	ui.mobileMemoryTab.classList.toggle("border-transparent", view !== "memory");
	ui.mobileMemoryTab.classList.toggle("text-muted", view !== "memory");
}

document.querySelectorAll("[data-memory-tab]").forEach((button) => {
	button.addEventListener("click", () => {
		state.memoryPoll?.abort();
		void loadMemory(button.dataset.memoryTab);
	});
});
ui.chatTab.addEventListener("click", () => setMobileView("chat"));
ui.mobileMemoryTab.addEventListener("click", () => setMobileView("memory"));
ui.messageForm.addEventListener("submit", (event) => {
	event.preventDefault();
	void sendMessage();
});
ui.messageInput.addEventListener("keydown", (event) => {
	if (event.key === "Enter" && !event.shiftKey && !event.isComposing && event.keyCode !== 229) {
		event.preventDefault();
		ui.messageForm.requestSubmit();
	}
});
window.addEventListener("resize", () => setMobileView(state.mobileView));

async function initialize() {
	createIcons({ icons });
	try {
		const serverState = await refreshState();
		await loadMessages();
		await loadMemory("daily");
		if (serverState.sending) void pollServerTurn();
		else if (serverState.pending_notes > 0) void pollMemoryExtraction();
	} catch (error) {
		showStatus(ui.chatStatus, `Noto could not start: ${error.message}`, "error");
	}
	setMobileView("chat");
}

void initialize();
