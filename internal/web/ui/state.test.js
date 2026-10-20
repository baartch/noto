import { describe, expect, test } from "bun:test";
import { clearSubmittedDraft, createPendingUserMessage, loadSendAttempt, mergeOlderMessages, pollUntil, saveSendAttempt, scrollToBottom, sendWithAttempt, sortMemoryOldestFirst } from "./state.js";

describe("chat UI state", () => {
	test("empty submits are rejected and uncertain failures reuse the same request ID", async () => {
		let ids = 0;
		const idFactory = () => `request-${++ids}`;
		const empty = await sendWithAttempt("  ", null, async () => ({ ok: true }), idFactory);
		expect(empty.attempt).toBe(null);
		expect(ids).toBe(0);

		const failed = await sendWithAttempt("hello", null, async () => { throw new Error("disconnected"); }, idFactory);
		expect(failed.attempt).toEqual({ requestID: "request-1", content: "hello" });
		const retry = await sendWithAttempt("hello", failed.attempt, async (attempt) => attempt, idFactory);
		expect(retry.response.requestID).toBe("request-1");
		expect(retry.attempt).toBe(null);
		const next = await sendWithAttempt("another", retry.attempt, async (attempt) => attempt, idFactory);
		expect(next.response.requestID).toBe("request-2");
	});

	test("a definitive API error allows an explicit new attempt", async () => {
		let ids = 0;
		const idFactory = () => `known-${++ids}`;
		const failed = await sendWithAttempt("hello", null, async () => {
			throw Object.assign(new Error("provider failed"), { status: 502 });
		}, idFactory);
		expect(failed.attempt).toBe(null);
		const manualRetry = await sendWithAttempt("hello", failed.attempt, async (attempt) => attempt, idFactory);
		expect(manualRetry.response.requestID).toBe("known-2");
	});

	test("older pages prepend chronologically without duplicate messages", () => {
		const current = [
			{ id: "m2", created_at: "2026-01-02T00:00:00Z" },
			{ id: "m3", created_at: "2026-01-03T00:00:00Z" },
		];
		const older = [
			{ id: "m1", created_at: "2026-01-01T00:00:00Z" },
			{ id: "m2", created_at: "2026-01-02T00:00:00Z" },
		];
		expect(mergeOlderMessages(current, older).map((item) => item.id)).toEqual(["m1", "m2", "m3"]);
	});

	test("notes and summaries are sorted oldest-first so newest stays at the bottom", () => {
		const notes = [
			{ id: "new", created_at: "2026-10-20T10:00:00Z" },
			{ id: "old", created_at: "2026-10-18T10:00:00Z" },
		];
		const summaries = [
			{ id: "new", period_start: "2026-10-01T00:00:00Z" },
			{ id: "old", period_start: "2026-09-01T00:00:00Z" },
		];
		expect(sortMemoryOldestFirst(notes, "daily").map((item) => item.id)).toEqual(["old", "new"]);
		expect(sortMemoryOldestFirst(summaries, "weekly").map((item) => item.id)).toEqual(["old", "new"]);
	});

	test("new chat text is represented immediately as a safe optimistic user message", () => {
		const pending = createPendingUserMessage({ requestID: "r1", content: "<b>hello</b>" }, new Date("2026-10-20T12:00:00Z"));
		expect(pending).toEqual({
			id: "pending:r1", role: "user", content: "<b>hello</b>", created_at: "2026-10-20T12:00:00.000Z", optimistic: true,
		});
	});

	test("chat scroll helper pins the viewport to the bottom", () => {
		const viewport = { scrollTop: 0, scrollHeight: 720 };
		scrollToBottom(viewport);
		expect(viewport.scrollTop).toBe(720);
	});

	test("a newer draft typed while a send is in flight is not cleared", () => {
		expect(clearSubmittedDraft("sent text", "sent text")).toBe("");
		expect(clearSubmittedDraft("new draft", "sent text")).toBe("new draft");
	});

	test("uncertain request IDs survive a page reload without triggering a resend", () => {
		const values = new Map();
		const storage = {
			getItem: (key) => values.get(key) ?? null,
			setItem: (key, value) => values.set(key, value),
			removeItem: (key) => values.delete(key),
		};
		const attempt = { requestID: "request-9", content: "message that may have reached the server" };
		saveSendAttempt(storage, attempt);
		expect(loadSendAttempt(storage)).toEqual(attempt);
		expect(values.size).toBe(1);
		saveSendAttempt(storage, null);
		expect(loadSendAttempt(storage)).toBe(null);
		expect(values.size).toBe(0);
	});

	test("polling stops when ready or at its bounded attempt count", async () => {
		let calls = 0;
		let delays = 0;
		const ready = await pollUntil(async () => ({ pending_notes: ++calls === 2 ? 0 : 1 }), (state) => state.pending_notes === 0, {
			maxAttempts: 60,
			sleepFn: async () => { delays++; },
		});
		expect(ready.pending_notes).toBe(0);
		expect(calls).toBe(2);
		expect(delays).toBe(1);

		calls = 0;
		const expired = await pollUntil(async () => { calls++; return { sending: true }; }, (state) => !state.sending, {
			maxAttempts: 3,
			sleepFn: async () => {},
		});
		expect(expired).toBe(null);
		expect(calls).toBe(3);
	});

	test("polling stops after its signal is aborted", async () => {
		const controller = new AbortController();
		let calls = 0;
		const result = await pollUntil(async () => { calls++; return { sending: true }; }, () => false, {
			maxAttempts: 10,
			signal: controller.signal,
			sleepFn: async () => { controller.abort(); },
		});
		expect(result).toBe(null);
		expect(calls).toBe(1);
	});
});
