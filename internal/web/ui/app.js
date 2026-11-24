import {
	Bot,
	ChevronDown,
	Copy,
	Info,
	LoaderCircle,
	Mic,
	MessagesSquare,
	NotebookPen,
	Pencil,
	Send,
	Settings,
	Sparkles,
	UserRound,
	X,
	createIcons,
} from "lucide";
import HSOverlay from "preline/plugins/overlay-non-auto";
import HSSelect from "preline/plugins/select-non-auto";
import HSTabs from "preline/plugins/tabs-non-auto";
import { copyRichTextToClipboard, createPendingUserMessage, loadSendAttempt, mergeOlderMessages, pollUntil, saveSendAttempt, scrollToBottom, sendWithAttempt, sortMemoryOldestFirst } from "./state.js";
import { createVoiceController, filterAvailableVoiceModels, LOCAL_VOICE_MODEL_ID, voiceReady } from "./voice.js";

const icons = { Bot, ChevronDown, Copy, Info, LoaderCircle, Mic, MessagesSquare, NotebookPen, Pencil, Send, Settings, Sparkles, UserRound, X };
const $ = (selector) => document.querySelector(selector);
const ui = {
	profile: $("#profile-name"),
	model: $("#model-name"),
	settingsButton: $("#settings-button"),
	settingsPanel: $("#settings-panel"),
	settingsTabs: $("#settings-tabs"),
	settingsClose: $("#settings-close"),
	settingsProfileTab: $("#settings-profile-tab"),
	settingsProfilePanel: $("#settings-profile-panel"),
	settingsModelsPanel: $("#settings-models-panel"),
	settingsVoiceTab: $("#settings-voice-tab"),
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
	voiceProvider: $("#voice-provider"),
	voiceProviderPanel: $("#voice-provider-panel"),
	voiceModel: $("#voice-model"),
	voiceModelSearch: $("#voice-model-search"),
	voiceModelOptions: $("#voice-model-options"),
	voiceModelHelp: $("#voice-model-help"),
	voiceModelRetry: $("#voice-model-retry"),
	voiceLocalPanel: $("#voice-local-panel"),
	voiceModelStatus: $("#voice-model-status"),
	voiceLocalReason: $("#voice-local-reason"),
	voiceDownloadProgress: $("#voice-download-progress"),
	voiceDownload: $("#voice-download"),
	voiceDelete: $("#voice-delete"),
	voiceReadyStatus: $("#voice-ready-status"),
	voiceDraftStatus: $("#voice-draft-status"),
	voiceSettingsStatus: $("#voice-settings-status"),
	voiceSave: $("#voice-save"),
	settingsStatus: $("#settings-status"),
	settingsSave: $("#settings-save"),
	settingsCancel: $("#settings-cancel"),
	setupBanner: $("#setup-banner"),
	chatStatus: $("#chat-status"),
	chatPanel: $("#chat-panel"),
	memoryPanel: $("#memory-panel"),
	memoryPanelContent: $("#memory-panel-content"),
	messageScroll: $("#message-scroll"),
	messageList: $("#message-list"),
	messageForm: $("#message-form"),
	messageInput: $("#message-input"),
	composerIdleControls: $("#composer-idle-controls"),
	sendButton: $("#send-button"),
	sendLabel: $("#send-label"),
	sendState: $("#send-state"),
	voiceRecord: $("#voice-record"),
	voiceReadinessHelp: $("#voice-readiness-help"),
	voiceReadinessMessage: $("#voice-readiness-message"),
	voiceOpenSettings: $("#voice-open-settings"),
	voiceRequestStatus: $("#voice-request-status"),
	voiceRecordingPanel: $("#voice-recording-panel"),
	voiceCancel: $("#voice-cancel"),
	voiceElapsed: $("#voice-elapsed"),
	voiceVolume: $("#voice-volume"),
	voiceVolumeFill: $("#voice-volume-fill"),
	voicePhaseLabel: $("#voice-phase-label"),
	voiceEdit: $("#voice-edit"),
	voiceSend: $("#voice-send"),
	voiceError: $("#voice-error"),
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
	settingsSavedModel: "",
	settingsSavedExtractorModel: "",
	settingsTab: "profile",
	voiceLoading: false,
	voiceLoaded: false,
	voiceSaving: false,
	voiceSaveGeneration: 0,
	voiceDTO: null,
	voiceReady: false,
	draftVoiceProvider: "",
	draftVoiceModel: "",
	savedVoiceProvider: "",
	savedVoiceModel: "",
	voiceDraftDirty: false,
	voiceModels: [],
	voiceModelsLoading: false,
	voiceModelsUnavailable: false,
	voiceModelsError: "",
	voiceModelStatus: null,
	voiceModelStatusReason: "",
	voiceOperationStarting: false,
	voiceRequestID: 0,
	voiceModelsRequestID: 0,
	voiceStatusRequestID: 0,
	voiceStatusTimer: null,
	composerVoiceDTO: null,
	composerVoiceLoading: false,
	composerVoiceError: "",
	composerVoiceRequestID: 0,
	voiceCapture: { phase: "idle", elapsedMs: 0, volume: 0, error: "", profileSlug: "" },
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
		void loadVoiceSettings(value);
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
const voiceModelDropdown = createSearchableDropdown({
	input: ui.voiceModelSearch, select: ui.voiceModel, listbox: ui.voiceModelOptions,
	listboxID: "voice-model-options",
	filterOptions(options, search) {
		const allowed = new Set(filterAvailableVoiceModels(options.map((option) => option.value), search));
		return options.filter((option) => allowed.has(option.value));
	},
	onChange(value) {
		state.draftVoiceModel = value;
		updateVoiceDraftDirty();
		renderVoiceSettings();
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
	ui.model.title = state.model;
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
	updateComposerVoiceControls();
	updateSettingsControls();
}

function voiceRecorderSupported() {
	return typeof navigator !== "undefined"
		&& typeof window !== "undefined"
		&& typeof navigator.mediaDevices?.getUserMedia === "function"
		&& typeof (window.AudioContext || window.webkitAudioContext) === "function";
}

function updateComposerVoiceControls() {
	const ready = voiceReady(state.composerVoiceDTO);
	const supported = voiceRecorderSupported();
	let reason = state.composerVoiceError;
	if (!reason && !supported) reason = "Microphone recording is not supported in this browser.";
	if (!reason && state.composerVoiceLoading) reason = "Checking voice dictation readiness…";
	if (!reason && !ready) {
		const detail = state.composerVoiceDTO?.reason;
		reason = detail ? `Voice is not ready. Open Settings: ${detail}` : "Voice must be configured in Settings before recording.";
	}
	const needsSetup = Boolean(reason);
	ui.voiceReadinessHelp.classList.toggle("hidden", !needsSetup);
	ui.voiceReadinessMessage.textContent = state.composerVoiceLoading
		? "Checking voice dictation readiness…"
		: reason;
	ui.voiceOpenSettings.classList.toggle("hidden", !needsSetup || !supported);
	ui.voiceRecord.title = needsSetup ? reason || "Checking voice readiness" : "Start voice recording";
	const phase = state.voiceCapture.phase;
	const voiceBusy = ["requesting", "recording", "processing"].includes(phase);
	ui.sendButton.disabled = state.setupRequired || state.sending || !state.profileSlug || state.recoveryRequired || voiceBusy;
	ui.voiceRecord.disabled = !ready || !supported || state.composerVoiceLoading || state.setupRequired || state.recoveryRequired || state.sending || !["idle", "error"].includes(phase);
	ui.voiceRequestStatus.textContent = phase === "requesting" ? "Waiting for microphone permission…" : "";
	ui.voiceRequestStatus.classList.toggle("hidden", phase !== "requesting");
}

function formatVoiceElapsed(elapsedMs) {
	const seconds = Math.floor(Math.max(0, elapsedMs) / 1000);
	return `${String(Math.floor(seconds / 60)).padStart(2, "0")}:${String(seconds % 60).padStart(2, "0")}`;
}

function updateVoiceRecordingState(captureState) {
	const previousPhase = state.voiceCapture.phase;
	const activeElement = document.activeElement;
	const hadVoiceFocus = ui.voiceRecordingPanel.contains(activeElement) || activeElement === ui.voiceRecord || activeElement === ui.messageInput;
	state.voiceCapture = { ...captureState };
	const requesting = captureState.phase === "requesting";
	const recording = captureState.phase === "recording";
	const processing = captureState.phase === "processing";
	const active = requesting || recording || processing;
	ui.messageInput.readOnly = ["requesting", "recording", "processing"].includes(captureState.phase);
	ui.composerIdleControls.classList.toggle("hidden", active);
	ui.voiceRecordingPanel.classList.toggle("hidden", !active);
	ui.voiceRecordingPanel.classList.toggle("flex", active);
	ui.voiceElapsed.textContent = formatVoiceElapsed(captureState.elapsedMs);
	ui.voiceVolume.setAttribute("aria-valuenow", String(Math.round(Math.max(0, Math.min(1, captureState.volume || 0)) * 100)));
	ui.voiceVolumeFill.style.transform = `scaleX(${Math.max(0, Math.min(1, captureState.volume || 0))})`;
	ui.voicePhaseLabel.textContent = requesting ? "Waiting for microphone permission" : processing ? "Transcribing recording" : "Recording";
	ui.voiceCancel.disabled = !["requesting", "recording", "processing"].includes(captureState.phase);
	ui.voiceEdit.disabled = !recording;
	ui.voiceSend.disabled = !recording;
	ui.voiceError.textContent = captureState.error || "";
	ui.voiceError.classList.toggle("hidden", !captureState.error);
	updateComposerVoiceControls();
	if (["idle", "error"].includes(previousPhase) && requesting) ui.messageInput.focus();
	else if (["requesting", "recording", "processing"].includes(previousPhase) && ["idle", "error"].includes(captureState.phase) && hadVoiceFocus) ui.messageInput.focus();
}

async function loadComposerVoiceReadiness(profileSlug = state.profileSlug) {
	const requestID = ++state.composerVoiceRequestID;
	state.composerVoiceLoading = true;
	state.composerVoiceError = "";
	updateComposerVoiceControls();
	if (!profileSlug) {
		state.composerVoiceLoading = false;
		state.composerVoiceDTO = null;
		updateComposerVoiceControls();
		return;
	}
	try {
		const query = new URLSearchParams({ profile_slug: profileSlug });
		const dto = await api(`/api/voice?${query}`);
		if (requestID !== state.composerVoiceRequestID || profileSlug !== state.profileSlug || dto.profile_slug !== profileSlug) return;
		state.composerVoiceDTO = dto;
	} catch (error) {
		if (requestID === state.composerVoiceRequestID && profileSlug === state.profileSlug) {
			state.composerVoiceDTO = null;
			state.composerVoiceError = `Voice readiness could not be checked: ${error.message}`;
		}
	} finally {
		if (requestID === state.composerVoiceRequestID) {
			state.composerVoiceLoading = false;
			updateComposerVoiceControls();
		}
	}
}

function applyComposerVoiceDTO(dto) {
	state.composerVoiceRequestID++;
	state.composerVoiceDTO = dto;
	state.composerVoiceError = "";
	state.composerVoiceLoading = false;
	updateComposerVoiceControls();
}

function updateSettingsControls() {
	const busy = state.settingsLoading || state.settingsSaving || state.voiceSaving || state.sending;
	const draftLocked = busy || state.settingsConflict;
	const voicePage = state.settingsTab === "voice";
	ui.settingsProfile.disabled = draftLocked;
	ui.settingsModel.disabled = draftLocked || state.settingsModelLoading || !state.settingsModelsLoaded || state.settingsSetupRequired || !ui.settingsModel.options?.length;
	ui.settingsEmbeddings.disabled = draftLocked || state.settingsModelLoading || !state.settingsModelsLoaded || state.settingsSetupRequired || !ui.settingsEmbeddings.options?.length;
	ui.settingsExtractor.disabled = draftLocked || state.settingsModelLoading || !state.settingsModelsLoaded || state.settingsSetupRequired || !ui.settingsExtractor.options?.length;
	ui.settingsProfileSearch.disabled = draftLocked;
	ui.settingsModelSearch.disabled = ui.settingsModel.disabled;
	ui.settingsEmbeddingsSearch.disabled = ui.settingsEmbeddings.disabled;
	ui.settingsExtractorSearch.disabled = ui.settingsExtractor.disabled;
	ui.settingsRetryModels.disabled = busy || state.settingsModelLoading;
	ui.voiceProvider.disabled = draftLocked || state.voiceLoading || state.settingsModelLoading || !state.voiceLoaded;
	ui.voiceModelSearch.disabled = draftLocked || state.voiceLoading || state.voiceModelsLoading || state.draftVoiceProvider !== "provider" || state.voiceModels.length === 0;
	ui.voiceModelRetry.disabled = busy || state.voiceModelsLoading || state.voiceLoading;
	ui.voiceSave.classList.toggle("hidden", !voicePage);
	ui.settingsSave.classList.toggle("hidden", voicePage);
	const voiceProviderModelMissing = state.draftVoiceProvider === "provider" && (!state.draftVoiceModel || !state.voiceModels.includes(state.draftVoiceModel));
	ui.voiceSave.disabled = draftLocked || state.voiceLoading || state.settingsModelLoading || !state.voiceLoaded || !state.voiceDraftDirty || !state.draftProfileSlug || state.draftProfileSlug !== state.profileSlug || state.recoveryRequired || state.voiceModelsLoading || voiceProviderModelMissing;
	ui.voiceSave.textContent = state.voiceSaving ? "Saving…" : "Save voice settings";
	const modelStatus = state.voiceModelStatus || state.voiceDTO?.model_status;
	ui.voiceDownload.disabled = state.voiceOperationStarting || !modelStatus?.available || modelStatus.installed || modelStatus.downloading || modelStatus.deleting;
	ui.voiceDelete.disabled = state.voiceOperationStarting || !modelStatus?.installed || modelStatus.downloading || modelStatus.deleting;
	const embeddingRebuildRequired = !state.settingsSetupRequired && state.draftEmbeddingsModel !== state.savedEmbeddingsModel && Boolean(state.draftEmbeddingsModel);
	ui.settingsEmbeddingWarning.classList.toggle("hidden", !embeddingRebuildRequired);
	ui.settingsSave.disabled = draftLocked || state.settingsModelLoading || !state.settingsModelsLoaded || !state.draftProfileSlug || state.recoveryRequired || (!state.settingsSetupRequired && !state.draftModel) || (embeddingRebuildRequired && !state.embeddingRebuildConfirmed);
	ui.settingsSave.textContent = state.settingsSaving ? "Saving…" : "Save";
	ui.settingsCancel.disabled = state.settingsSaving || state.voiceSaving || state.voiceOperationStarting;
	ui.settingsClose.disabled = state.settingsSaving || state.voiceSaving || state.voiceOperationStarting;
}

function setSettingsPage(open) {
	state.settingsOpen = open;
	ui.settingsButton.setAttribute("aria-expanded", String(open));
	if (open) {
		state.settingsTab = "profile";
		HSTabs.open(ui.settingsProfileTab);
		if (HSOverlay.getInstance(ui.settingsPanel)) HSOverlay.open(ui.settingsPanel);
		else ui.settingsPanel.classList.remove("hidden");
	} else {
		state.settingsTab = "profile";
		if (HSOverlay.getInstance(ui.settingsPanel)) HSOverlay.close(ui.settingsPanel);
		else ui.settingsPanel.classList.add("hidden");
		ui.settingsButton.focus();
	}
}

function focusSettingsField() {
	const focusTarget = state.settingsTab === "voice"
		? ui.voiceProvider
		: ui.settingsModelsPanel.classList.contains("hidden") ? ui.settingsProfileSearch : ui.settingsModelSearch;
	focusTarget.focus();
}

function syncSettingsTab() {
	if (ui.settingsVoiceTab.getAttribute("aria-selected") === "true") state.settingsTab = "voice";
	else if (ui.settingsModelsPanel.classList.contains("hidden")) state.settingsTab = "profile";
	else state.settingsTab = "models";
	updateSettingsControls();
	if (state.settingsTab === "voice") {
		if (!state.voiceLoaded && state.draftProfileSlug && !state.voiceLoading) void loadVoiceSettings(state.draftProfileSlug);
		else void loadVoiceModelStatus();
	}
}

function updateSettingsTabOrientation() {
	ui.settingsTabs.setAttribute("aria-orientation", window.matchMedia("(min-width: 768px)").matches ? "vertical" : "horizontal");
}

function trapSettingsTab(event) {
	if (event.key !== "Tab" || !state.settingsOpen) return;
	const focusable = Array.from(ui.settingsPanel.querySelectorAll("a[href]:not([tabindex='-1']), button:not([disabled]):not([tabindex='-1']), input:not([disabled]):not([tabindex='-1']), select:not([disabled]):not([tabindex='-1']), textarea:not([disabled]):not([tabindex='-1']), [tabindex]:not([tabindex='-1'])"))
		.filter((element) => element.getClientRects().length > 0 && !element.closest(".hidden, [hidden], [aria-hidden='true']"));
	if (!focusable.length) return;
	const currentIndex = focusable.indexOf(document.activeElement);
	const direction = event.shiftKey ? -1 : 1;
	const nextIndex = currentIndex < 0
		? (direction < 0 ? focusable.length - 1 : 0)
		: (currentIndex + direction + focusable.length) % focusable.length;
	event.preventDefault();
	event.stopPropagation();
	focusable[nextIndex].focus();
}

function settingsOption(value, label) {
	const option = document.createElement("option");
	option.value = value;
	option.textContent = label;
	return option;
}

function createSearchableDropdown({ input, select, listbox, listboxID, onChange, filterOptions }) {
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
		matches = filterOptions
			? filterOptions(options, query)
			: options.filter((option) => !query || `${option.label} ${option.value}`.toLocaleLowerCase().includes(query));
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
			if (listbox.classList.contains("hidden")) return;
			event.preventDefault();
			event.stopPropagation?.();
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
		state.settingsSavedModel = details.saved_model || "";
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
		state.settingsSavedExtractorModel = savedExtractorModel;
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


function updateVoiceDraftDirty() {
	state.voiceDraftDirty = state.draftVoiceProvider !== state.savedVoiceProvider || state.draftVoiceModel !== state.savedVoiceModel;
	renderVoiceSettings();
	updateSettingsControls();
}

function renderVoiceSettings() {
	const provider = state.draftVoiceProvider;
	ui.voiceProvider.value = provider;
	ui.voiceProviderPanel.classList.toggle("hidden", provider !== "provider");
	ui.voiceLocalPanel.classList.toggle("hidden", provider !== "local");
	state.voiceReady = voiceReady(state.voiceDTO);
	ui.voiceReadyStatus.textContent = state.voiceLoading
		? "Loading voice settings…"
		: state.voiceReady
			? "Ready for dictation."
			: state.voiceDTO?.reason || "Voice is not configured and ready.";

	let draftMessage = "";
	if (state.draftProfileSlug && state.draftProfileSlug !== state.profileSlug) {
		draftMessage = "Switch to this profile before saving its voice settings.";
	} else if (state.voiceDraftDirty) {
		draftMessage = "Unsaved voice changes. Save to apply this selection.";
	}
	ui.voiceDraftStatus.textContent = draftMessage;
	ui.voiceDraftStatus.classList.toggle("hidden", !draftMessage);

	if (provider === "provider") {
		if (state.voiceModelsLoading) {
			ui.voiceModelHelp.textContent = "Loading verified speech-to-text models…";
		} else if (state.voiceModelsUnavailable) {
			ui.voiceModelHelp.textContent = state.voiceModelsError || "Speech-to-text models are unavailable.";
		} else if (state.voiceModels.length === 0) {
			ui.voiceModelHelp.textContent = "No verified speech-to-text models are available. Check provider support and retry.";
		} else if (state.draftVoiceModel && !state.voiceModels.includes(state.draftVoiceModel)) {
			ui.voiceModelHelp.textContent = "The saved speech model is not in this provider’s verified transcription list.";
		} else {
			ui.voiceModelHelp.textContent = "Choose a verified transcription model; the chat model is not used for speech.";
		}
		ui.voiceModelRetry.classList.toggle("hidden", provider !== "provider" || state.voiceModelsLoading || (!state.voiceModelsUnavailable && state.voiceModels.length > 0));
	}

	const status = state.voiceModelStatus || state.voiceDTO?.model_status || null;
	if (!status) {
		ui.voiceModelStatus.textContent = state.voiceLoading ? "Checking local model status…" : "Local model status is unavailable.";
		ui.voiceLocalReason.textContent = "";
		ui.voiceDownloadProgress.classList.add("hidden");
		ui.voiceDownload.classList.add("hidden");
		ui.voiceDelete.classList.add("hidden");
		return;
	}
	const progress = Math.max(0, Math.min(1, Number(status.progress) || 0));
	if (!status.available) ui.voiceModelStatus.textContent = "Local speech runtime is unavailable on this platform.";
	else if (status.deleting) ui.voiceModelStatus.textContent = "Deleting the local speech model…";
	else if (status.downloading) ui.voiceModelStatus.textContent = `Downloading the local model (${Math.round(progress * 100)}%).`;
	else if (status.installed) ui.voiceModelStatus.textContent = status.verified ? "Installed and verified." : "Installed; integrity will be checked before first use.";
	else ui.voiceModelStatus.textContent = "Not installed.";
	ui.voiceLocalReason.textContent = state.voiceModelStatusReason || (provider === "local" && !state.voiceReady ? state.voiceDTO?.reason || "" : "");
	ui.voiceDownloadProgress.value = progress;
	ui.voiceDownloadProgress.classList.toggle("hidden", !status.downloading);
	ui.voiceDownload.classList.toggle("hidden", !status.available || status.installed);
	ui.voiceDownload.disabled = state.voiceOperationStarting || !status.available || status.downloading || status.deleting;
	ui.voiceDownload.textContent = status.downloading ? `Downloading (${Math.round(progress * 100)}%)…` : "Download model";
	ui.voiceDelete.classList.toggle("hidden", !status.installed);
	ui.voiceDelete.disabled = state.voiceOperationStarting || status.downloading || status.deleting;
}

async function loadVoiceSettings(profileSlug, preserveDraft = false) {
	const requestID = ++state.voiceRequestID;
	const saveGeneration = state.voiceSaveGeneration;
	state.voiceModelsRequestID++;
	const keepDraft = preserveDraft && state.voiceDraftDirty;
	const savedDraftProvider = state.draftVoiceProvider;
	const savedDraftModel = state.draftVoiceModel;
	const expectedRevision = state.draftRevision;
	state.voiceLoading = true;
	state.voiceLoaded = false;
	state.voiceModels = [];
	state.voiceModelsLoading = false;
	state.voiceModelsUnavailable = false;
	state.voiceModelsError = "";
	voiceModelDropdown.setOptions([], "");
	renderVoiceSettings();
	updateSettingsControls();
	if (!profileSlug) {
		state.voiceLoading = false;
		renderVoiceSettings();
		updateSettingsControls();
		return;
	}
	try {
		const query = new URLSearchParams({ profile_slug: profileSlug });
		const dto = await api(`/api/voice?${query}`);
		if (requestID !== state.voiceRequestID || saveGeneration !== state.voiceSaveGeneration || state.voiceSaving || profileSlug !== state.draftProfileSlug || dto.profile_slug !== profileSlug) return;
		const responseRevision = Number(dto.revision || 0);
		if (expectedRevision > 0 && responseRevision > 0 && responseRevision !== expectedRevision) {
			state.settingsConflict = true;
			showStatus(ui.voiceSettingsStatus, "Settings changed in another tab. Reload the latest settings before saving.", "error", { label: "Reload", run: reloadSettings });
			return;
		}
		state.voiceDTO = dto;
		if (profileSlug === state.profileSlug) applyComposerVoiceDTO(dto);
		state.savedVoiceProvider = dto.provider || "";
		state.savedVoiceModel = dto.model || "";
		state.voiceLoaded = true;
		if (keepDraft) {
			state.draftVoiceProvider = savedDraftProvider;
			state.draftVoiceModel = savedDraftModel;
			state.voiceDraftDirty = true;
		} else {
			state.draftVoiceProvider = state.savedVoiceProvider;
			state.draftVoiceModel = state.savedVoiceModel;
			state.voiceDraftDirty = false;
		}
		state.voiceModelStatus = dto.model_status || state.voiceModelStatus;
		state.voiceModelStatusReason = "";
		state.voiceReady = voiceReady(dto);
		renderVoiceSettings();
		if (state.draftVoiceProvider === "provider") void loadVoiceModels(profileSlug, requestID, responseRevision || expectedRevision);
		void loadVoiceModelStatus();
	} catch (error) {
		if (requestID === state.voiceRequestID) {
			state.voiceLoaded = false;
			showStatus(ui.voiceSettingsStatus, `Could not load voice settings: ${error.message}`, "error");
		}
	} finally {
		if (requestID === state.voiceRequestID) {
			state.voiceLoading = false;
			renderVoiceSettings();
			updateSettingsControls();
		}
	}
}

async function loadVoiceModels(profileSlug, profileRequestID = state.voiceRequestID, expectedRevision = state.draftRevision) {
	const requestID = ++state.voiceModelsRequestID;
	state.voiceModelsLoading = true;
	state.voiceModelsUnavailable = false;
	state.voiceModelsError = "";
	state.voiceModels = [];
	voiceModelDropdown.setOptions([], "");
	renderVoiceSettings();
	updateSettingsControls();
	try {
		const query = new URLSearchParams({ profile_slug: profileSlug });
		const result = await api(`/api/voice/models?${query}`);
		if (requestID !== state.voiceModelsRequestID || profileRequestID !== state.voiceRequestID || profileSlug !== state.draftProfileSlug || result.profile_slug !== profileSlug) return;
		const responseRevision = Number(result.revision || 0);
		if (expectedRevision > 0 && responseRevision > 0 && responseRevision !== expectedRevision) {
			state.settingsConflict = true;
			state.voiceModelsUnavailable = true;
			state.voiceModelsError = "Settings changed in another tab. Reload before choosing a speech model.";
			showStatus(ui.voiceSettingsStatus, state.voiceModelsError, "error", { label: "Reload", run: reloadSettings });
			return;
		}
		state.voiceModels = filterAvailableVoiceModels(result.models, "");
		voiceModelDropdown.setOptions(state.voiceModels.map((model) => ({ value: model, label: model })), state.voiceModels.includes(state.draftVoiceModel) ? state.draftVoiceModel : "");
	} catch (error) {
		if (requestID === state.voiceModelsRequestID && profileRequestID === state.voiceRequestID) {
			state.voiceModels = [];
			state.voiceModelsUnavailable = true;
			state.voiceModelsError = error.message || "Speech-to-text models are unavailable.";
			voiceModelDropdown.setOptions([], "");
		}
	} finally {
		if (requestID === state.voiceModelsRequestID) {
			state.voiceModelsLoading = false;
			renderVoiceSettings();
			updateSettingsControls();
		}
	}
}

function scheduleVoiceModelStatusPoll() {
	if (state.voiceStatusTimer) window.clearTimeout(state.voiceStatusTimer);
	state.voiceStatusTimer = window.setTimeout(() => {
		state.voiceStatusTimer = null;
		void loadVoiceModelStatus();
	}, 750);
}

async function loadVoiceModelStatus() {
	const requestID = ++state.voiceStatusRequestID;
	try {
		const result = await api("/api/voice/local-model");
		if (requestID !== state.voiceStatusRequestID || !state.settingsOpen || !result.model_status) return;
		const wasDownloading = Boolean(state.voiceModelStatus?.downloading);
		state.voiceModelStatus = result.model_status;
		state.voiceModelStatusReason = result.reason || "";
		renderVoiceSettings();
		updateSettingsControls();
		if (result.model_status.downloading || result.model_status.deleting) scheduleVoiceModelStatusPoll();
		else {
			if (state.voiceStatusTimer) window.clearTimeout(state.voiceStatusTimer);
			state.voiceStatusTimer = null;
			if (wasDownloading && state.draftProfileSlug) void loadVoiceSettings(state.draftProfileSlug, true);
		}
	} catch (error) {
		if (requestID === state.voiceStatusRequestID && state.settingsOpen) {
			state.voiceModelStatusReason = error.message || "Local model status is unavailable.";
			renderVoiceSettings();
		}
	}
}

async function startVoiceModelDownload() {
	if (ui.voiceDownload.disabled || state.voiceOperationStarting) return;
	state.voiceOperationStarting = true;
	showStatus(ui.voiceSettingsStatus, "Starting local model download…");
	renderVoiceSettings();
	updateSettingsControls();
	try {
		const result = await api("/api/voice/local-model/download", { method: "POST" });
		state.voiceModelStatus = result.model_status || state.voiceModelStatus;
		state.voiceModelStatusReason = result.reason || "";
		showStatus(ui.voiceSettingsStatus, "Local model download started.");
		if (state.voiceModelStatus?.downloading) scheduleVoiceModelStatusPoll();
		else if (state.draftProfileSlug) void loadVoiceSettings(state.draftProfileSlug, true);
	} catch (error) {
		showStatus(ui.voiceSettingsStatus, `Could not start local model download: ${error.message}`, "error");
	} finally {
		state.voiceOperationStarting = false;
		renderVoiceSettings();
		updateSettingsControls();
	}
}

async function deleteVoiceModel() {
	if (ui.voiceDelete.disabled || state.voiceOperationStarting) return;
	if (!window.confirm("Delete the shared Parakeet model for all profiles? Local dictation will remain unavailable until it is downloaded again.")) return;
	state.voiceOperationStarting = true;
	showStatus(ui.voiceSettingsStatus, "Deleting the shared local model…");
	renderVoiceSettings();
	updateSettingsControls();
	try {
		const result = await api("/api/voice/local-model", { method: "DELETE" });
		state.voiceModelStatus = result.model_status || state.voiceModelStatus;
		state.voiceModelStatusReason = result.reason || "";
		showStatus(ui.voiceSettingsStatus, "Local model deleted for all profiles.");
		if (state.draftProfileSlug) void loadVoiceSettings(state.draftProfileSlug, true);
	} catch (error) {
		showStatus(ui.voiceSettingsStatus, `Could not delete local model: ${error.message}`, "error");
	} finally {
		state.voiceOperationStarting = false;
		renderVoiceSettings();
		updateSettingsControls();
	}
}


async function saveVoiceSettings(event) {
	event?.preventDefault();
	if (ui.voiceSave.disabled || state.voiceSaving) return;
	const profileSlug = state.draftProfileSlug;
	if (!profileSlug || profileSlug !== state.profileSlug) {
		showStatus(ui.voiceSettingsStatus, "Switch to the selected profile before saving its voice settings.", "error");
		return;
	}
	state.voiceSaving = true;
	state.voiceSaveGeneration++;
	showStatus(ui.voiceSettingsStatus, "Saving voice settings…");
	updateSettingsControls();
	try {
		const dto = await api("/api/voice", {
			method: "POST",
			headers: { "Content-Type": "application/json" },
			body: JSON.stringify({ profile_slug: profileSlug, provider: state.draftVoiceProvider, model: state.draftVoiceModel, revision: state.draftRevision }),
		});
		if (dto.profile_slug !== profileSlug) return;
		if (state.draftProfileSlug !== profileSlug || state.profileSlug !== profileSlug) {
			state.settingsConflict = true;
			showStatus(ui.voiceSettingsStatus, "The active profile changed while saving. Reload settings before continuing.", "error", { label: "Reload", run: reloadSettings });
			return;
		}
		const savedRevision = Number(dto.revision || 0);
		if (savedRevision > 0 && savedRevision < state.revision) {
			state.settingsConflict = true;
			showStatus(ui.voiceSettingsStatus, "Settings changed in another tab. Reload before continuing.", "error", { label: "Reload", run: reloadSettings });
			return;
		}
		state.voiceDTO = dto;
		applyComposerVoiceDTO(dto);
		state.savedVoiceProvider = dto.provider || "";
		state.savedVoiceModel = dto.model || "";
		state.draftVoiceProvider = state.savedVoiceProvider;
		state.draftVoiceModel = state.savedVoiceModel;
		state.voiceDraftDirty = false;
		state.voiceLoaded = true;
		state.voiceReady = voiceReady(dto);
		if (savedRevision > 0) {
			state.revision = savedRevision;
			state.draftRevision = savedRevision;
		}
		showStatus(ui.voiceSettingsStatus, "Voice settings saved.");
		renderVoiceSettings();
		if (state.draftVoiceProvider === "provider") void loadVoiceModels(profileSlug, state.voiceRequestID, state.draftRevision);
	} catch (error) {
		if (error.code === "settings_changed" || error.code === "profile_changed") {
			state.settingsConflict = true;
			showStatus(ui.voiceSettingsStatus, "Settings changed in another tab. Reload before saving voice settings.", "error", { label: "Reload", run: reloadSettings });
		} else {
			showStatus(ui.voiceSettingsStatus, `Could not save voice settings: ${error.message}`, "error");
		}
	} finally {
		state.voiceSaving = false;
		state.voiceSaveGeneration++;
		renderVoiceSettings();
		updateSettingsControls();
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
		focusSettingsField();
		void loadVoiceSettings(state.draftProfileSlug);
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
		focusSettingsField();
		void loadVoiceSettings(state.draftProfileSlug);
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
	if (state.settingsSaving || state.voiceSaving) return;
	closeSettings();
}

function closeSettings() {
	state.settingsOpenRequestID++;
	state.settingsModelRequestID++;
	state.voiceRequestID++;
	state.voiceModelsRequestID++;
	state.voiceStatusRequestID++;
	if (state.voiceStatusTimer) window.clearTimeout(state.voiceStatusTimer);
	state.voiceStatusTimer = null;
	state.settingsModelController?.abort();
	state.settingsModelController = null;
	state.settingsModelLoading = false;
	state.settingsLoading = false;
	state.settingsModelsLoaded = false;
	state.settingsConflict = false;
	state.voiceLoading = false;
	state.voiceLoaded = false;
	state.voiceModelsLoading = false;
	state.voiceModelsUnavailable = false;
	state.voiceModelsError = "";
	state.voiceModels = [];
	state.voiceDTO = null;
	state.voiceModelStatus = null;
	state.voiceModelStatusReason = "";
	state.voiceOperationStarting = false;
	state.voiceDraftDirty = false;
	state.draftVoiceProvider = "";
	state.draftVoiceModel = "";
	state.savedVoiceProvider = "";
	state.savedVoiceModel = "";
	state.draftProfileSlug = "";
	state.draftModel = "";
	state.draftEmbeddingsModel = "";
	state.draftExtractorModel = "";
	state.savedEmbeddingsModel = "";
	state.settingsEmbeddingModelsUnavailable = false;
	state.embeddingRebuildConfirmed = false;
	ui.settingsConfirmEmbeddingRebuild.checked = false;
	showStatus(ui.settingsStatus, "");
	showStatus(ui.voiceSettingsStatus, "");
	setSettingsPage(false);
}

async function saveSettings(event) {
	event?.preventDefault();
	if (state.settingsTab === "voice") {
		await saveVoiceSettings(event);
		return;
	}
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
	copyButton.className = `inline-flex size-8 shrink-0 items-center justify-center rounded-full border border-transparent p-0 focus-visible:ring-2 focus-visible:ring-focus ${isUser ? "text-white/80 hover:bg-white/10 hover:text-white" : "text-muted hover:bg-raised hover:text-ink"}`;
	copyButton.setAttribute("aria-label", "Copy message");
	copyButton.title = "Copy message";
	copyButton.innerHTML = '<i data-lucide="copy" class="size-4" aria-hidden="true"></i>';
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
	card.className = "min-w-0 rounded-lg border border-line bg-raised p-4 shadow-sm";
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
		button.classList.toggle("active", selected);
		button.setAttribute("aria-selected", String(selected));
		button.classList.toggle("bg-active", selected);
		button.classList.toggle("text-ink", selected);
		button.classList.toggle("text-muted", !selected);
	}
	ui.memoryPanelContent.setAttribute("aria-labelledby", `memory-tab-${state.memoryTab}`);
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
	if (profileChanged) {
		clearProfileScopedData();
		if (["requesting", "recording", "processing"].includes(state.voiceCapture.phase)) {
			voiceController.cancel();
			showStatus(ui.chatStatus, "The active profile changed. The recording was discarded and your draft was kept.", "error");
		}
	}
	if (state.settingsOpen && revisionChanged && !state.settingsSaving) {
		state.settingsConflict = true;
		state.settingsModelsLoaded = false;
		showStatus(ui.settingsStatus, "Settings changed in another tab. Reload the latest settings before saving.", "error", { label: "Reload settings", run: reloadSettings });
		updateSettingsControls();
	}
	renderState(serverState);
	if (profileChanged) void loadComposerVoiceReadiness(profileSlug);
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
	if (state.setupRequired || state.sending || !state.profileSlug || ["requesting", "recording", "processing"].includes(state.voiceCapture.phase)) return;
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
		void loadComposerVoiceReadiness();
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

const voiceController = createVoiceController({
	getUserMedia: (constraints) => navigator.mediaDevices.getUserMedia(constraints),
	audioContextFactory: () => {
		const AudioContextConstructor = window.AudioContext || window.webkitAudioContext;
		if (!AudioContextConstructor) throw new Error("This browser does not support microphone recording.");
		return new AudioContextConstructor();
	},
	transcribe: async (wav, profileSlug, signal) => {
		const query = new URLSearchParams({ profile_slug: profileSlug });
		const result = await api(`/api/voice/transcriptions?${query}`, {
			method: "POST",
			headers: { "Content-Type": "audio/wav" },
			body: wav,
			signal,
		});
		return result?.text || "";
	},
	getDraft: () => ui.messageInput.value,
	getProfileSlug: () => state.profileSlug,
	onInsert: (text) => {
		ui.messageInput.value = text;
		ui.messageInput.readOnly = false;
		ui.messageInput.focus();
		ui.messageInput.setSelectionRange?.(text.length, text.length);
	},
	onSend: async (text, profileSlug) => {
		if (profileSlug !== state.profileSlug) throw new Error("The active profile changed. The recording was discarded.");
		ui.messageInput.value = text;
		await sendMessage();
	},
	onState: updateVoiceRecordingState,
});

function handleVoiceKeydown(event) {
	if (state.voiceCapture.phase !== "recording" || event.isComposing || event.keyCode === 229) return;
	const targetIsControl = event.target !== ui.messageInput && Boolean(event.target?.closest?.("button, a, input, select, textarea"));
	if (targetIsControl && event.key !== "Escape") return;
	const hasModifier = event.shiftKey || event.ctrlKey || event.altKey || event.metaKey;
	if (event.key === "Escape") {
		event.preventDefault();
		event.stopPropagation();
		voiceController.cancel();
	} else if (!hasModifier && (event.key === "." || event.code === "Period")) {
		event.preventDefault();
		event.stopPropagation();
		void voiceController.finish("edit");
	} else if (!hasModifier && event.key === "Enter") {
		event.preventDefault();
		event.stopPropagation();
		void voiceController.finish("send");
	}
}

ui.settingsButton.addEventListener("click", openSettings);
ui.voiceOpenSettings.addEventListener("click", openSettings);
ui.voiceRecord.addEventListener("click", () => {
	void voiceController.start(state.profileSlug).catch(() => { /* The live region reports permission and platform errors. */ });
});
ui.voiceCancel.addEventListener("click", () => voiceController.cancel());
ui.voiceEdit.addEventListener("click", () => void voiceController.finish("edit"));
ui.voiceSend.addEventListener("click", () => void voiceController.finish("send"));
document.addEventListener("keydown", handleVoiceKeydown, true);
window.addEventListener("pagehide", (event) => {
	if (event.persisted) voiceController.cancel();
	else void voiceController.dispose();
});
ui.settingsCancel.addEventListener("click", cancelSettings);
ui.settingsClose.addEventListener("click", cancelSettings);
ui.settingsForm.addEventListener("submit", (event) => {
	if (state.settingsTab === "voice") {
		return saveVoiceSettings(event);
	}
	return saveSettings(event);
});
ui.settingsTabs.addEventListener("click", () => queueMicrotask(syncSettingsTab));
ui.settingsTabs.addEventListener("keydown", () => window.setTimeout(syncSettingsTab, 0));
ui.voiceProvider.addEventListener("change", () => {
	const previousProvider = state.draftVoiceProvider;
	state.draftVoiceProvider = ui.voiceProvider.value;
	if (state.draftVoiceProvider === "local") {
		state.draftVoiceModel = LOCAL_VOICE_MODEL_ID;
		state.voiceModels = [];
		voiceModelDropdown.setOptions([], "");
	} else if (state.draftVoiceProvider === "provider") {
		state.draftVoiceModel = previousProvider === "provider" || state.savedVoiceProvider === "provider" ? state.savedVoiceModel : "";
		if (state.draftProfileSlug) void loadVoiceModels(state.draftProfileSlug, state.voiceRequestID, state.draftRevision);
	} else {
		state.draftVoiceModel = "";
		state.voiceModels = [];
		voiceModelDropdown.setOptions([], "");
	}
	updateVoiceDraftDirty();
});
ui.voiceModelRetry.addEventListener("click", () => {
	if (state.draftProfileSlug) void loadVoiceModels(state.draftProfileSlug, state.voiceRequestID, state.draftRevision);
});
ui.voiceSave.addEventListener("click", (event) => void saveVoiceSettings(event));
ui.voiceDownload.addEventListener("click", () => void startVoiceModelDownload());
ui.voiceDelete.addEventListener("click", () => void deleteVoiceModel());
ui.settingsPanel.addEventListener("keydown", trapSettingsTab);
document.addEventListener("keydown", (event) => {
	if (event.key !== "Escape" || !state.settingsOpen) return;
	event.preventDefault();
	cancelSettings();
});
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
	if (["requesting", "recording", "processing"].includes(state.voiceCapture.phase)) return;
	void sendMessage();
});
ui.messageInput.addEventListener("keydown", (event) => {
	if (event.key === "Enter" && !event.shiftKey && !event.isComposing && event.keyCode !== 229) {
		if (["requesting", "recording", "processing"].includes(state.voiceCapture.phase)) {
			event.preventDefault();
			event.stopPropagation();
			return;
		}
		event.preventDefault();
		ui.messageForm.requestSubmit();
	}
});
window.addEventListener("resize", () => {
	setMobileView(state.mobileView);
	updateSettingsTabOrientation();
});

async function initialize() {
	createIcons({ icons });
	updateSettingsTabOrientation();
	HSOverlay.autoInit();
	HSTabs.autoInit();
	HSSelect.autoInit();
	try {
		const serverState = await refreshState();
		void loadComposerVoiceReadiness();
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
