import {
	ArrowUp,
	Bot,
	ChevronDown,
	Copy,
	Info,
	LoaderCircle,
	MessagesSquare,
	NotebookPen,
	Settings,
	Sparkles,
	UserRound,
	createIcons,
} from "lucide";
import HSOverlay from "preline/plugins/overlay-non-auto";
import HSSelect from "preline/plugins/select-non-auto";
import HSTabs from "preline/plugins/tabs-non-auto";
import { copyRichTextToClipboard, createPendingUserMessage, loadSendAttempt, mergeOlderMessages, pollUntil, saveSendAttempt, scrollToBottom, sendWithAttempt, sortMemoryOldestFirst } from "./state.js";

const icons = { ArrowUp, Bot, ChevronDown, Copy, Info, LoaderCircle, MessagesSquare, NotebookPen, Settings, Sparkles, UserRound };
const $ = (selector) => document.querySelector(selector);
const ui = {
	profile: $("#profile-name"),
	model: $("#model-name"),
	settingsButton: $("#settings-button"),
	workspace: $("#workspace"),
	workspaceNav: $("#workspace-nav"),
	settingsPanel: $("#settings-panel"),
	settingsForm: $("#settings-form"),
	settingsProfile: $("#settings-profile"),
	settingsProfileSearch: $("#settings-profile-search"),
	settingsProfileOptions: $("#settings-profile-options"),
	settingsModel: $("#settings-model"),
	settingsModelSearch: $("#settings-model-search"),
	settingsModelOptions: $("#settings-model-options"),
	settingsModelHelp: $("#settings-model-help"),
	settingsEmbeddings: $("#settings-embeddings"),
	settingsEmbeddingsSearch: $("#settings-embeddings-search"),
	settingsEmbeddingsOptions: $("#settings-embeddings-options"),
	settingsEmbeddingsHelp: $("#settings-embeddings-help"),
	settingsEmbeddingsInfoButton: $("#settings-embeddings-info-button"),
	settingsEmbeddingsInfo: $("#settings-embeddings-info"),
	settingsEmbeddingWarning: $("#settings-embedding-warning"),
	settingsConfirmEmbeddingRebuild: $("#settings-confirm-embedding-rebuild"),
	settingsExtractor: $("#settings-extractor"),
	settingsExtractorSearch: $("#settings-extractor-search"),
	settingsExtractorOptions: $("#settings-extractor-options"),
	settingsExtractorHelp: $("#settings-extractor-help"),
	settingsRetryModels: $("#settings-retry-models"),
	settingsStatus: $("#settings-status"),
	settingsSave: $("#settings-save"),
	settingsCancel: $("#settings-cancel"),
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
	profileSlug: "",
	model: "",
	revision: 0,
	recoveryRequired: false,
	setupRequired: true,
	sending: false,
	pendingNotes: 0,
	messages: [],
	pendingUserMessage: null,
	nextCursor: null,
	loadingOlder: false,
	messageRequestID: 0,
	memoryTab: "daily",
	memoryOffset: 0,
	memoryNextOffset: null,
	memoryItems: [],
	memoryRequest: null,
	memoryPoll: null,
	statePoll: null,
	sendAttempt: null,
	mobileView: "chat",
	settingsOpen: false,
	settingsLoading: false,
	settingsModelLoading: false,
	settingsSaving: false,
	settingsModelsLoaded: false,
	settingsChatModelsUnavailable: false,
	settingsEmbeddingModelsUnavailable: false,
	settingsSetupRequired: false,
	settingsConflict: false,
	settingsProfiles: [],
	draftProfileSlug: "",
	draftModel: "",
	draftEmbeddingsModel: "",
	draftExtractorModel: "",
	savedEmbeddingsModel: "",
	embeddingRebuildConfirmed: false,
	draftRevision: 0,
	settingsModelRequestID: 0,
	settingsOpenRequestID: 0,
	settingsModelController: null,
	profileSyncing: false,
	profileSyncTimer: null,
	profileSyncController: null,
};

const searchableDropdowns = [];
const settingsProfileDropdown = createSearchableDropdown({
	input: ui.settingsProfileSearch, select: ui.settingsProfile, listbox: ui.settingsProfileOptions,
	listboxID: "settings-profile-options",
	onChange(value) {
		state.draftProfileSlug = value;
		state.settingsModelsLoaded = false;
		return loadSettingsModels(value);
	},
});
const settingsModelDropdown = createSearchableDropdown({
	input: ui.settingsModelSearch, select: ui.settingsModel, listbox: ui.settingsModelOptions,
	listboxID: "settings-model-options",
	onChange(value) {
		state.draftModel = value;
		updateSettingsControls();
	},
});
const settingsEmbeddingsDropdown = createSearchableDropdown({
	input: ui.settingsEmbeddingsSearch, select: ui.settingsEmbeddings, listbox: ui.settingsEmbeddingsOptions,
	listboxID: "settings-embeddings-options",
	onChange(value) {
		state.draftEmbeddingsModel = value;
		state.embeddingRebuildConfirmed = false;
		ui.settingsConfirmEmbeddingRebuild.checked = false;
		updateSettingsControls();
	},
});
const settingsExtractorDropdown = createSearchableDropdown({
	input: ui.settingsExtractorSearch, select: ui.settingsExtractor, listbox: ui.settingsExtractorOptions,
	listboxID: "settings-extractor-options",
	onChange(value) {
		state.draftExtractorModel = value;
		updateSettingsControls();
	},
});
ui.settingsEmbeddingsInfo.classList.add("hidden");
ui.settingsEmbeddingsInfoButton.setAttribute("aria-expanded", "false");
ui.settingsEmbeddingsInfoButton.setAttribute("aria-label", "Why does changing the embeddings model rebuild the index?");
ui.settingsEmbeddingWarning.classList.add("hidden");
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
	state.profileSlug = serverState.profile?.slug || "";
	state.model = serverState.model || "Provider not configured";
	state.revision = Number(serverState.revision || 0);
	state.recoveryRequired = Boolean(serverState.recovery_required);
	state.setupRequired = Boolean(serverState.setup_required);
	state.sending = Boolean(serverState.sending);
	state.pendingNotes = Number(serverState.pending_notes || 0);
	ui.profile.textContent = state.profile;
	ui.model.textContent = state.model;
	updateSetupBanner();
	updateSendControls();
}

function updateSetupBanner() {
	ui.setupBanner.classList.toggle("hidden", !state.setupRequired && !state.recoveryRequired);
	if (state.recoveryRequired) {
		ui.setupBanner.textContent = "Settings require recovery; chat sends and settings changes are disabled. Contact your administrator before continuing.";
	} else {
		ui.setupBanner.textContent = "Noto needs a configured provider before you can chat. Set it up in the terminal, then restart noto web.";
	}
}

function updateSendControls() {
	const disabled = state.setupRequired || state.sending || !state.profileSlug || state.recoveryRequired;
	ui.sendButton.disabled = disabled;
	ui.messageInput.disabled = state.setupRequired || state.recoveryRequired;
	ui.sendLabel.textContent = state.sending ? "Thinking…" : "Send";
	ui.sendState.textContent = state.sending ? "Noto is working" : state.pendingNotes > 0 ? "Saving memory…" : "";
	updateSettingsControls();
}

function updateSettingsControls() {
	const busy = state.settingsLoading || state.settingsSaving || state.sending;
	const draftLocked = busy || state.settingsConflict;
	ui.settingsProfile.disabled = draftLocked;
	ui.settingsModel.disabled = draftLocked || state.settingsModelLoading || !state.settingsModelsLoaded || state.settingsSetupRequired || !ui.settingsModel.options?.length;
	ui.settingsEmbeddings.disabled = draftLocked || state.settingsModelLoading || !state.settingsModelsLoaded || state.settingsSetupRequired || !ui.settingsEmbeddings.options?.length;
	ui.settingsExtractor.disabled = draftLocked || state.settingsModelLoading || !state.settingsModelsLoaded || state.settingsSetupRequired || !ui.settingsExtractor.options?.length;
	ui.settingsProfileSearch.disabled = draftLocked;
	ui.settingsModelSearch.disabled = ui.settingsModel.disabled;
	ui.settingsEmbeddingsSearch.disabled = ui.settingsEmbeddings.disabled;
	ui.settingsExtractorSearch.disabled = ui.settingsExtractor.disabled;
	ui.settingsRetryModels.disabled = busy || state.settingsModelLoading;
	const embeddingRebuildRequired = !state.settingsSetupRequired && state.draftEmbeddingsModel !== state.savedEmbeddingsModel && Boolean(state.draftEmbeddingsModel);
	ui.settingsEmbeddingWarning.classList.toggle("hidden", !embeddingRebuildRequired);
	ui.settingsSave.disabled = draftLocked || state.settingsModelLoading || !state.settingsModelsLoaded || !state.draftProfileSlug || state.recoveryRequired || (!state.settingsSetupRequired && !state.draftModel) || (embeddingRebuildRequired && !state.embeddingRebuildConfirmed);
	ui.settingsSave.textContent = state.settingsSaving ? "Saving…" : "Save";
	ui.settingsCancel.disabled = state.settingsSaving;
}

function setSettingsPage(open) {
	state.settingsOpen = open;
	ui.settingsButton.setAttribute("aria-expanded", String(open));
	ui.workspace.classList.toggle("hidden", open);
	ui.workspaceNav.classList.toggle("hidden", open);
	ui.settingsPanel.classList.toggle("hidden", !open);
	if (!open) {
		setMobileView(state.mobileView);
		ui.settingsButton.focus();
	}
}

function settingsOption(value, label) {
	const option = document.createElement("option");
	option.value = value;
	option.textContent = label;
	return option;
}

function createSearchableDropdown({ input, select, listbox, listboxID, onChange }) {
	let options = [];
	let matches = [];
	let activeIndex = -1;
	input.setAttribute("role", "combobox");
	input.setAttribute("aria-autocomplete", "list");
	input.setAttribute("aria-haspopup", "listbox");
	input.setAttribute("aria-controls", listboxID);
	input.setAttribute("aria-expanded", "false");
	listbox.setAttribute("role", "listbox");
	listbox.addEventListener("mousedown", (event) => event.preventDefault());
	const selectedOption = () => options.find((option) => option.value === select.value);
	const close = () => {
		listbox.classList.add("hidden");
		input.setAttribute("aria-expanded", "false");
		input.setAttribute("aria-activedescendant", "");
		activeIndex = -1;
	};
	const updateActiveOption = () => {
		Array.from(listbox.children).forEach((item, index) => item.classList.toggle("bg-raised", index === activeIndex));
		input.setAttribute("aria-activedescendant", activeIndex >= 0 ? `${listboxID}-option-${activeIndex}` : "");
		if (activeIndex >= 0) listbox.children[activeIndex]?.scrollIntoView?.({ block: "nearest" });
	};
	const choose = (option) => {
		if (!option) return;
		select.value = option.value;
		input.value = option.label;
		close();
		onChange(option.value);
	};
	const setOptions = (nextOptions, selectedValue) => {
		options = nextOptions.map(({ value, label }) => ({ value: String(value), label: String(label) }));
		select.replaceChildren(...options.map((option) => settingsOption(option.value, option.label)));
		select.value = selectedValue == null ? "" : String(selectedValue);
		input.value = selectedOption()?.label || "";
		close();
	};
	const render = () => {
		const query = input.value.trim().toLocaleLowerCase();
		matches = options.filter((option) => !query || `${option.label} ${option.value}`.toLocaleLowerCase().includes(query));
		activeIndex = matches.length > 0 ? 0 : -1;
		listbox.replaceChildren();
		if (matches.length === 0) {
			const empty = document.createElement("div");
			empty.setAttribute("role", "option");
			empty.setAttribute("aria-selected", "false");
			empty.setAttribute("aria-disabled", "true");
			empty.setAttribute("tabindex", "-1");
			empty.className = "px-3 py-2 text-sm text-muted";
			empty.textContent = "No matching options.";
			listbox.append(empty);
		} else {
			for (const [index, option] of matches.entries()) {
				const item = document.createElement("button");
				item.type = "button";
				item.id = `${listboxID}-option-${index}`;
				item.setAttribute("role", "option");
				item.setAttribute("aria-selected", String(option.value === select.value));
				item.setAttribute("tabindex", "-1");
				item.className = "block w-full rounded-lg px-3 py-2 text-left text-sm text-ink hover:bg-raised focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus";
				item.textContent = option.label;
				item.addEventListener("mousedown", (event) => event.preventDefault());
				item.addEventListener("click", () => choose(option));
				listbox.append(item);
			}
		}
		listbox.classList.remove("hidden");
		input.setAttribute("aria-expanded", "true");
		updateActiveOption();
	};
	input.addEventListener("focus", () => {
		for (const dropdown of searchableDropdowns) dropdown.close();
		input.select?.();
		render();
	});
	input.addEventListener("click", () => {
		if (listbox.classList.contains("hidden")) render();
	});
	input.addEventListener("input", render);
	input.addEventListener("blur", () => {
		close();
		input.value = selectedOption()?.label || "";
	});
	input.addEventListener("keydown", (event) => {
		if (event.key === "Escape") {
			event.preventDefault();
			close();
			input.value = selectedOption()?.label || "";
			return;
		}
		if (event.key === "ArrowDown" || event.key === "ArrowUp") {
			event.preventDefault();
			if (listbox.classList.contains("hidden")) render();
			if (matches.length === 0) return;
			const delta = event.key === "ArrowDown" ? 1 : -1;
			activeIndex = (activeIndex + delta + matches.length) % matches.length;
			updateActiveOption();
			return;
		}
		if (event.key === "Enter" && !listbox.classList.contains("hidden")) {
			event.preventDefault();
			choose(matches[activeIndex] || matches[0]);
		}
	});
	select.addEventListener("change", () => {
		input.value = selectedOption()?.label || "";
		close();
		return onChange(select.value);
	});
	const dropdown = { setOptions, close };
	searchableDropdowns.push(dropdown);
	return dropdown;
}

function renderSettingsProfiles() {
	settingsProfileDropdown.setOptions(state.settingsProfiles.map((profile) => ({ value: profile.slug, label: profile.name })), state.draftProfileSlug);
}

async function loadSettingsModels(profileSlug) {
	state.settingsModelController?.abort();
	const controller = new AbortController();
	const requestID = ++state.settingsModelRequestID;
	state.settingsModelController = controller;
	state.settingsModelLoading = true;
	state.settingsModelsLoaded = false;
	state.settingsChatModelsUnavailable = false;
	state.draftModel = "";
	state.draftEmbeddingsModel = "";
	state.draftExtractorModel = "";
	state.savedEmbeddingsModel = "";
	state.settingsEmbeddingModelsUnavailable = false;
	state.embeddingRebuildConfirmed = false;
	state.settingsSetupRequired = false;
	settingsModelDropdown.setOptions([], "");
	settingsEmbeddingsDropdown.setOptions([], "");
	settingsExtractorDropdown.setOptions([], "");
	ui.settingsConfirmEmbeddingRebuild.checked = false;
	ui.settingsRetryModels.classList.add("hidden");
	ui.settingsModelHelp.textContent = "Loading available models…";
	ui.settingsEmbeddingsHelp.textContent = "Loading embeddings models…";
	ui.settingsExtractorHelp.textContent = "Loading extractor models…";
	showStatus(ui.settingsStatus, "");
	updateSettingsControls();
	if (!profileSlug) {
		state.settingsModelLoading = false;
		ui.settingsModelHelp.textContent = "Choose a profile to load its available models.";
		updateSettingsControls();
		return;
	}
	try {
		const query = new URLSearchParams({ profile: profileSlug });
		const details = await api(`/api/settings/models?${query}`, { signal: controller.signal });
		if (controller.signal.aborted || requestID !== state.settingsModelRequestID || profileSlug !== state.draftProfileSlug || details.profile_slug !== profileSlug) return;
		const availableModels = [...(details.models || [])];
		const models = [...availableModels];
		if (details.saved_model && !models.includes(details.saved_model)) models.unshift(details.saved_model);
		settingsModelDropdown.setOptions(models.map((model) => ({
			value: model,
			label: model === details.saved_model && (details.models_unavailable || !availableModels.includes(model)) ? `${model} (saved, unavailable)` : model,
		})), details.saved_model || models[0] || "");
		state.draftModel = details.saved_model || models[0] || "";
		state.settingsChatModelsUnavailable = Boolean(details.models_unavailable);
		const savedEmbeddingsModel = details.saved_embeddings_model || "";
		state.savedEmbeddingsModel = savedEmbeddingsModel;
		state.draftEmbeddingsModel = savedEmbeddingsModel;
		state.settingsEmbeddingModelsUnavailable = Boolean(details.embedding_models_unavailable);
		const embeddingModels = [...(details.embedding_models || [])];
		if (savedEmbeddingsModel && !embeddingModels.includes(savedEmbeddingsModel)) embeddingModels.unshift(savedEmbeddingsModel);
		settingsEmbeddingsDropdown.setOptions([
			{ value: "", label: "Disabled" },
			...embeddingModels.map((model) => ({ value: model, label: model === savedEmbeddingsModel && !(details.embedding_models || []).includes(model) ? `${model} (saved, unavailable)` : model })),
		], savedEmbeddingsModel);
		const savedExtractorModel = details.saved_extractor_model || "";
		const extractorModels = [...availableModels];
		if (savedExtractorModel && !extractorModels.includes(savedExtractorModel)) extractorModels.unshift(savedExtractorModel);
		settingsExtractorDropdown.setOptions([
			{ value: "", label: "Use chat model (fallback)" },
			...extractorModels.map((model) => ({ value: model, label: model === savedExtractorModel && (details.models_unavailable || !availableModels.includes(model)) ? `${model} (saved, unavailable)` : model })),
		], savedExtractorModel);
		state.draftExtractorModel = savedExtractorModel;
		state.settingsSetupRequired = Boolean(details.setup_required);
		state.settingsModelsLoaded = true;
		ui.settingsRetryModels.classList.toggle("hidden", !state.settingsChatModelsUnavailable && !state.settingsEmbeddingModelsUnavailable);
		ui.settingsModelHelp.textContent = state.settingsSetupRequired
			? "This profile has no configured provider. You can save the profile and configure chat later in the terminal."
			: state.settingsChatModelsUnavailable
				? "Chat model choices could not be loaded. Saved chat and extractor choices are kept; retry to load more."
			: "The selected model is saved with this profile.";
		ui.settingsEmbeddingsHelp.textContent = state.settingsEmbeddingModelsUnavailable
			? "Embeddings model choices could not be loaded. Your saved choice is kept; clearing it remains available."
			: "Used to turn memories and searches into vectors.";
		ui.settingsExtractorHelp.textContent = savedExtractorModel
			? "Used to extract memories from chat."
			: "No extractor model is saved; the chat model is used as the fallback.";
		showStatus(ui.settingsStatus, "");
	} catch (error) {
		if (error.name !== "AbortError" && requestID === state.settingsModelRequestID) {
			ui.settingsModelHelp.textContent = "Model choices could not be loaded.";
			ui.settingsRetryModels.classList.remove("hidden");
			showStatus(ui.settingsStatus, `Could not load models: ${error.message}`, "error");
		}
	} finally {
		if (requestID === state.settingsModelRequestID) {
			state.settingsModelLoading = false;
			state.settingsModelController = null;
			updateSettingsControls();
		}
	}
}

async function openSettings() {
	if (state.settingsSaving || state.settingsOpen) return;
	const requestID = ++state.settingsOpenRequestID;
	setSettingsPage(true);
	state.settingsLoading = true;
	state.settingsModelsLoaded = false;
	state.settingsChatModelsUnavailable = false;
	state.draftModel = "";
	state.draftRevision = state.revision;
	showStatus(ui.settingsStatus, "Loading settings…");
	updateSettingsControls();
	try {
		const settings = await api("/api/settings");
		if (!state.settingsOpen || requestID !== state.settingsOpenRequestID) return;
		state.settingsProfiles = settings.profiles || [];
		state.draftProfileSlug = settings.active_profile_slug || state.profileSlug;
		renderSettingsProfiles();
		state.settingsLoading = false;
		updateSettingsControls();
		ui.settingsProfileSearch.focus();
		await loadSettingsModels(state.draftProfileSlug);
		if (state.recoveryRequired) showStatus(ui.settingsStatus, "Settings require recovery; further changes are disabled.", "error");
	} catch (error) {
		if (state.settingsOpen && requestID === state.settingsOpenRequestID) showStatus(ui.settingsStatus, `Could not load settings: ${error.message}`, "error", { label: "Retry", run: () => void reloadSettings() });
	} finally {
		if (requestID === state.settingsOpenRequestID) {
			state.settingsLoading = false;
			updateSettingsControls();
		}
	}
}

async function reloadSettings() {
	if (!state.settingsOpen || state.settingsSaving) return;
	const requestID = ++state.settingsOpenRequestID;
	state.settingsConflict = false;
	state.settingsLoading = true;
	state.settingsModelsLoaded = false;
	showStatus(ui.settingsStatus, "Reloading latest settings…");
	updateSettingsControls();
	try {
		const previousProfileSlug = state.profileSlug;
		const serverState = await refreshState();
		if (previousProfileSlug && previousProfileSlug !== serverState.profile?.slug) {
			await loadMessages();
			if (serverState.pending_notes > 0) void pollMemoryExtraction();
			else await loadMemory(state.memoryTab);
		}
		const settings = await api("/api/settings");
		if (!state.settingsOpen || requestID !== state.settingsOpenRequestID) return;
		state.settingsConflict = false;
		showStatus(ui.settingsStatus, "");
		state.settingsProfiles = settings.profiles || [];
		state.draftProfileSlug = settings.active_profile_slug || serverState.profile?.slug || "";
		state.draftRevision = serverState.revision;
		renderSettingsProfiles();
		state.settingsLoading = false;
		updateSettingsControls();
		ui.settingsProfileSearch.focus();
		await loadSettingsModels(state.draftProfileSlug);
	} catch (error) {
		if (state.settingsOpen && requestID === state.settingsOpenRequestID) showStatus(ui.settingsStatus, `Could not reload settings: ${error.message}`, "error", { label: "Retry", run: () => void reloadSettings() });
	} finally {
		if (requestID === state.settingsOpenRequestID) {
			state.settingsLoading = false;
			updateSettingsControls();
		}
	}
}

function cancelSettings() {
	if (state.settingsSaving) return;
	closeSettings();
}

function closeSettings() {
	state.settingsOpenRequestID++;
	state.settingsModelRequestID++;
	state.settingsModelController?.abort();
	state.settingsModelController = null;
	state.settingsModelLoading = false;
	state.settingsLoading = false;
	state.settingsModelsLoaded = false;
	state.settingsConflict = false;
	state.draftProfileSlug = "";
	state.draftModel = "";
	state.draftEmbeddingsModel = "";
	state.draftExtractorModel = "";
	state.savedEmbeddingsModel = "";
	state.settingsEmbeddingModelsUnavailable = false;
	state.embeddingRebuildConfirmed = false;
	ui.settingsConfirmEmbeddingRebuild.checked = false;
	showStatus(ui.settingsStatus, "");
	setSettingsPage(false);
}

async function saveSettings(event) {
	event?.preventDefault();
	if (ui.settingsSave.disabled || state.sending || state.settingsSaving) return;
	state.settingsSaving = true;
	updateSettingsControls();
	try {
		const savedState = await api("/api/settings", {
			method: "POST",
			headers: { "Content-Type": "application/json" },
			body: JSON.stringify({
				profile_slug: state.draftProfileSlug,
				model: state.draftModel,
				embeddings_model: state.draftEmbeddingsModel,
				extractor_model: state.draftExtractorModel,
				revision: state.draftRevision,
			}),
		});
		const profileChanged = savedState.profile?.slug !== state.profileSlug;
		if (profileChanged) {
			state.memoryRequest?.abort();
			state.memoryPoll?.abort();
			state.statePoll?.abort();
			state.messages = [];
			state.pendingUserMessage = null;
			state.nextCursor = null;
			state.loadingOlder = false;
			state.memoryItems = [];
			state.memoryOffset = 0;
			state.memoryNextOffset = null;
			state.sendAttempt = null;
			saveSendAttempt(attemptStorage, null);
			renderMessages();
			renderMemory();
		}
		renderState(savedState);
		closeSettings();
		await loadMessages();
		if (savedState.pending_notes > 0) void pollMemoryExtraction();
		else await loadMemory(state.memoryTab);
	} catch (error) {
		showStatus(ui.settingsStatus, error.message || "Settings could not be saved.", "error");
		if (error.code === "settings_recovery_required") {
			state.recoveryRequired = true;
			updateSetupBanner();
			updateSendControls();
		} else if (error.code === "settings_changed") {
			state.settingsConflict = true;
			state.settingsModelsLoaded = false;
			showStatus(ui.settingsStatus, "Settings changed in another tab. Reload the latest settings before saving.", "error", { label: "Reload settings", run: reloadSettings });
			updateSettingsControls();
		}
	} finally {
		state.settingsSaving = false;
		updateSettingsControls();
	}
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
	row.className = isUser
		? "max-w-2xl min-w-0 ms-auto flex justify-end gap-x-2 sm:gap-x-4"
		: "flex min-w-0 gap-x-2 sm:gap-x-4";
	row.setAttribute("aria-label", isUser ? "Your message" : "Noto response");
	const avatar = document.createElement("span");
	avatar.className = `mt-0.5 inline-flex size-9.5 shrink-0 items-center justify-center ${isUser ? "rounded-full bg-active text-ink" : "rounded-lg bg-accent text-white"}`;
	avatar.setAttribute("data-message-avatar", isUser ? "user" : "assistant");
	avatar.setAttribute("aria-hidden", "true");
	avatar.innerHTML = isUser
		? '<i data-lucide="user-round" class="size-4" aria-hidden="true"></i>'
		: '<i data-lucide="bot" class="size-5" aria-hidden="true"></i>';
	const content = document.createElement("div");
	content.className = `min-w-0 space-y-3 ${isUser ? "grow text-end" : "w-full grow max-w-[90%] md:max-w-2xl"}`;
	const bubble = document.createElement("div");
	bubble.className = `inline-block max-w-full rounded-lg p-4 shadow-2xs ${isUser ? "bg-accent text-white" : "border border-line bg-panel"}`;
	bubble.setAttribute("data-message-bubble", isUser ? "user" : "assistant");
	const meta = document.createElement("div");
	meta.className = `mb-2 flex items-center gap-2 font-mono text-[10px] uppercase tracking-wide ${isUser ? "justify-end text-white/75" : "text-muted"}`;
	const name = document.createElement("span");
	name.textContent = isUser ? "You" : "Noto";
	const timestamp = document.createElement("time");
	timestamp.dateTime = message.created_at || "";
	timestamp.textContent = formatDate(message.created_at, { hour: "2-digit", minute: "2-digit" });
	meta.append(name, document.createTextNode("·"), timestamp);
	const body = document.createElement("div");
	body.className = `markdown-body break-words text-start text-sm leading-6 ${isUser ? "message-user-content text-white" : "text-ink"}`;
	body.setAttribute("data-message-content", "");
	if (message.optimistic && message.content_html === undefined) {
		body.classList.add("whitespace-pre-wrap");
		body.textContent = message.content;
	} else body.innerHTML = message.content_html || "";
	const copyButton = document.createElement("button");
	copyButton.type = "button";
	copyButton.className = `inline-flex items-center gap-x-2 rounded-full border border-transparent px-3 py-2 text-sm focus-visible:ring-2 focus-visible:ring-focus ${isUser ? "text-white/80 hover:bg-white/10 hover:text-white" : "text-muted hover:bg-raised hover:text-ink"}`;
	copyButton.setAttribute("aria-label", "Copy message");
	copyButton.title = "Copy message";
	copyButton.innerHTML = '<i data-lucide="copy" class="size-4" aria-hidden="true"></i>Copy';
	copyButton.addEventListener("click", () => void copyMessageContent(body, copyButton));
	const actions = document.createElement("div");
	actions.className = `mt-2 flex ${isUser ? "justify-end" : "justify-start"}`;
	actions.append(copyButton);
	bubble.append(meta, body, actions);
	content.append(bubble);
	if (isUser) row.append(content, avatar);
	else row.append(avatar, content);
	return row;
}

async function copyMessageContent(body, button) {
	try {
		await copyRichTextToClipboard(navigator.clipboard, body.innerHTML, body.innerText);
		button.title = "Copied";
		button.setAttribute("aria-label", "Message copied");
	} catch {
		showStatus(ui.chatStatus, "Could not copy message to clipboard.", "error");
	}
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
	const requestID = ++state.messageRequestID;
	state.loadingOlder = older;
	const profileSlug = state.profileSlug;
	const revision = state.revision;
	const previousHeight = ui.messageScroll.scrollHeight;
	const previousTop = ui.messageScroll.scrollTop;
	try {
		const query = new URLSearchParams({ limit: "50" });
		if (older) query.set("before", state.nextCursor);
		const page = await api(`/api/messages?${query}`);
		if (requestID !== state.messageRequestID || profileSlug !== state.profileSlug || revision !== state.revision) return;
		state.messages = older ? mergeOlderMessages(state.messages, page.items) : page.items;
		state.nextCursor = page.next_cursor;
		renderMessages({ scrollToBottom: !older });
		if (older) ui.messageScroll.scrollTop = previousTop + (ui.messageScroll.scrollHeight - previousHeight);
	} catch (error) {
		showStatus(ui.chatStatus, `Could not load conversation: ${error.message}`, "error");
	} finally {
		if (requestID === state.messageRequestID) state.loadingOlder = false;
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
	const profileSlug = state.profileSlug;
	const revision = state.revision;
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
		if (profileSlug !== state.profileSlug || revision !== state.revision) return;
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
	const { stale } = applyServerState(serverState);
	if (!stale) return serverState;
	return {
		...serverState,
		profile: { name: state.profile, slug: state.profileSlug },
		model: state.model,
		revision: state.revision,
		setup_required: state.setupRequired,
		sending: state.sending,
		pending_notes: state.pendingNotes,
		recovery_required: state.recoveryRequired,
	};
}

function clearProfileScopedData() {
	state.memoryRequest?.abort();
	state.memoryRequest = null;
	state.memoryPoll?.abort();
	state.memoryPoll = null;
	state.statePoll?.abort();
	state.statePoll = null;
	state.messageRequestID++;
	state.messages = [];
	state.pendingUserMessage = null;
	state.nextCursor = null;
	state.loadingOlder = false;
	state.memoryItems = [];
	state.memoryOffset = 0;
	state.memoryNextOffset = null;
	showStatus(ui.chatStatus, "");
	showStatus(ui.memoryStatus, "");
	renderMessages();
	renderMemory();
}

function applyServerState(serverState) {
	const profileSlug = serverState.profile?.slug || "";
	const revision = Number(serverState.revision || 0);
	if (state.revision > 0 && revision < state.revision) return { profileChanged: false, revisionChanged: false, stale: true };
	const profileChanged = Boolean(state.profileSlug && profileSlug && profileSlug !== state.profileSlug);
	const revisionChanged = revision !== state.revision;
	if (profileChanged) clearProfileScopedData();
	if (state.settingsOpen && revisionChanged && !state.settingsSaving) {
		state.settingsConflict = true;
		state.settingsModelsLoaded = false;
		showStatus(ui.settingsStatus, "Settings changed in another tab. Reload the latest settings before saving.", "error", { label: "Reload settings", run: reloadSettings });
		updateSettingsControls();
	}
	renderState(serverState);
	return { profileChanged, revisionChanged, stale: false };
}

async function checkActiveProfile() {
	if (state.profileSyncing || state.settingsSaving) return;
	state.profileSyncing = true;
	const controller = new AbortController();
	state.profileSyncController = controller;
	const timeout = window.setTimeout(() => controller.abort(), 5000);
	try {
		const serverState = await api("/api/state", { signal: controller.signal });
		if (controller.signal.aborted) return;
		const { profileChanged, stale } = applyServerState(serverState);
		if (stale) return;
		if (profileChanged) {
			await loadMessages();
			if (serverState.pending_notes > 0) void pollMemoryExtraction();
			else await loadMemory(state.memoryTab);
		}
	} catch (error) {
		if (error.name !== "AbortError") showStatus(ui.chatStatus, `Could not sync profile state: ${error.message}`, "error");
	} finally {
		window.clearTimeout(timeout);
		if (state.profileSyncController === controller) {
			state.profileSyncController = null;
			state.profileSyncing = false;
		}
	}
}

function startProfileSync() {
	const onVisible = () => {
		if (document.visibilityState === "visible") return checkActiveProfile();
	};
	if (typeof window.setInterval === "function" && state.profileSyncTimer === null) {
		state.profileSyncTimer = window.setInterval(onVisible, 2000);
	}
	window.addEventListener("focus", onVisible);
	document.addEventListener("visibilitychange", onVisible);
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
			const { stale } = applyServerState(settled);
			if (!stale) {
				state.pendingUserMessage = null;
				await loadMessages();
				if (settled.pending_notes > 0) void pollMemoryExtraction();
				else await loadMemory(state.memoryTab);
			}
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
			const { profileChanged, stale } = applyServerState(settled);
			if (!stale) {
				if (profileChanged) await loadMessages();
				await loadMemory(state.memoryTab);
			}
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
	if (state.setupRequired || state.sending || !state.profileSlug) return;
	const content = ui.messageInput.value;
	if (!content.trim()) return;
	ui.messageInput.value = "";
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
		void api("/api/markdown", {
			method: "POST",
			headers: { "Content-Type": "application/json" },
			body: JSON.stringify({ content: attempt.content }),
		}).then((preview) => {
			if (state.pendingUserMessage?.id !== `pending:${attempt.requestID}`) return;
			state.pendingUserMessage.content_html = preview.content_html;
			renderMessages();
		}).catch(() => { /* Keep the whitespace-preserving plain text preview. */ });
		return api("/api/messages", {
			method: "POST",
			headers: { "Content-Type": "application/json" },
			body: JSON.stringify({ request_id: attempt.requestID, content: attempt.content, profile_slug: attempt.profileSlug }),
		});
	}, undefined, state.profileSlug);
	state.sending = false;
	state.sendAttempt = result.attempt;
	saveSendAttempt(attemptStorage, result.attempt);
	if (result.error) {
		if (!ui.messageInput.value) ui.messageInput.value = content;
		if (!result.attempt) state.pendingUserMessage = null;
		updateSendControls();
		showStatus(ui.chatStatus, `${result.error.message}. The message was not sent again automatically.`, "error");
		try {
			const serverState = await refreshState();
			if (serverState.sending) void pollServerTurn();
			else {
				state.pendingUserMessage = null;
				await loadMessages();
				if (serverState.pending_notes > 0) void pollMemoryExtraction();
				else await loadMemory(state.memoryTab);
			}
		} catch {
			// Keep the draft and request ID so a manual retry cannot duplicate an accepted turn.
		}
		renderMessages({ scrollToBottom: true });
		return;
	}
	state.pendingUserMessage = null;
	state.sendAttempt = null;
	updateSendControls();
	try {
		const serverState = await refreshState();
		await loadMessages();
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

ui.settingsButton.addEventListener("click", openSettings);
ui.settingsCancel.addEventListener("click", cancelSettings);
ui.settingsForm.addEventListener("submit", saveSettings);
ui.settingsConfirmEmbeddingRebuild.addEventListener("change", () => {
	state.embeddingRebuildConfirmed = ui.settingsConfirmEmbeddingRebuild.checked;
	updateSettingsControls();
});
ui.settingsEmbeddingsInfoButton.addEventListener("click", () => {
	const open = ui.settingsEmbeddingsInfo.classList.contains("hidden");
	ui.settingsEmbeddingsInfo.classList.toggle("hidden", !open);
	ui.settingsEmbeddingsInfoButton.setAttribute("aria-expanded", String(open));
});
ui.settingsRetryModels.addEventListener("click", async () => await loadSettingsModels(state.draftProfileSlug));

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
	HSOverlay.autoInit();
	HSTabs.autoInit();
	HSSelect.autoInit();
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
	startProfileSync();
}

void initialize();
