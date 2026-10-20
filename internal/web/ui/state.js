const sleep = (milliseconds, signal) => new Promise((resolve) => {
	if (signal?.aborted) return resolve();
	const timer = setTimeout(finish, milliseconds);
	function finish() {
		clearTimeout(timer);
		signal?.removeEventListener("abort", finish);
		resolve();
	}
	signal?.addEventListener("abort", finish, { once: true });
});
const sendAttemptStorageKey = "noto:web:pending-send";
const requestIDPattern = /^[A-Za-z0-9_-]{1,64}$/;

function newRequestID() {
	if (globalThis.crypto?.randomUUID) return globalThis.crypto.randomUUID();
	return `${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

export function createSendAttempt(content, previous = null, idFactory = newRequestID) {
	if (typeof content !== "string" || content.trim() === "") return null;
	if (previous?.content === content) return previous;
	return { requestID: idFactory(), content };
}

export function saveSendAttempt(storage, attempt) {
	try {
		if (!attempt) storage.removeItem(sendAttemptStorageKey);
		else storage.setItem(sendAttemptStorageKey, JSON.stringify(attempt));
	} catch {
		// Storage can be disabled by the browser; chat remains usable in this tab.
	}
}

export function loadSendAttempt(storage) {
	try {
		const raw = storage.getItem(sendAttemptStorageKey);
		if (!raw) return null;
		const value = JSON.parse(raw);
		if (!requestIDPattern.test(value?.requestID) || typeof value.content !== "string" || !value.content.trim() || value.content.length > 65536) {
			storage.removeItem(sendAttemptStorageKey);
			return null;
		}
		return { requestID: value.requestID, content: value.content };
	} catch {
		try { storage.removeItem(sendAttemptStorageKey); } catch { /* Storage remains unavailable. */ }
		return null;
	}
}

export async function sendWithAttempt(content, previous, post, idFactory = newRequestID) {
	const attempt = createSendAttempt(content, previous, idFactory);
	if (!attempt) return { attempt: null, response: null, error: new Error("message is empty") };
	try {
		return { attempt: null, response: await post(attempt), error: null };
	} catch (error) {
		return { attempt, response: null, error };
	}
}

export function mergeOlderMessages(current, older) {
	const seen = new Set();
	const merged = [];
	for (const message of [...older, ...current]) {
		if (seen.has(message.id)) continue;
		seen.add(message.id);
		merged.push(message);
	}
	return merged;
}

export function clearSubmittedDraft(currentDraft, submittedContent) {
	return currentDraft === submittedContent ? "" : currentDraft;
}

export async function pollUntil(fetchState, isReady, { maxAttempts, intervalMs = 1000, sleepFn = sleep, signal }) {
	for (let attempt = 0; attempt < maxAttempts; attempt++) {
		if (signal?.aborted) return null;
		const state = await fetchState(signal);
		if (isReady(state)) return state;
		if (attempt + 1 < maxAttempts) await sleepFn(intervalMs, signal);
	}
	return null;
}
