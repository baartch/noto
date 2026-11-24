import { describe, expect, test } from "bun:test";
import { createVoiceController, encodeWAV, filterAvailableVoiceModels, resampleTo16k, voiceReady } from "./voice.js";

const localModel = "parakeet-tdt-0.6b-v3-int8";

function voiceDTO(overrides = {}) {
	return {
		profile_slug: "artist",
		provider: "",
		model: "",
		ready: false,
		model_status: { available: false, installed: false },
		...overrides,
	};
}

describe("voiceReady", () => {
	test("unconfigured selection is not ready", () => {
		expect(voiceReady(voiceDTO())).toBe(false);
	});

	test("local selection requires an available runtime and installed model", () => {
		expect(voiceReady(voiceDTO({
			provider: "local",
			model: localModel,
			ready: true,
			model_status: { available: true, installed: false },
		}))).toBe(false);
		expect(voiceReady(voiceDTO({
			provider: "local",
			model: localModel,
			ready: true,
			model_status: { available: false, installed: true },
		}))).toBe(false);
	});

	test("installed local Parakeet is ready only for its matching model ID", () => {
		const status = { available: true, installed: true };
		expect(voiceReady(voiceDTO({ provider: "local", model: localModel, ready: true, model_status: status }))).toBe(true);
		expect(voiceReady(voiceDTO({ provider: "local", model: "other-model", ready: true, model_status: status }))).toBe(false);
	});

	test("provider selection requires a model and backend readiness", () => {
		expect(voiceReady(voiceDTO({ provider: "provider", model: "whisper-1", ready: true }))).toBe(true);
		expect(voiceReady(voiceDTO({ provider: "provider", model: "", ready: true }))).toBe(false);
		expect(voiceReady(voiceDTO({ provider: "provider", model: "whisper-1", ready: false }))).toBe(false);
	});
});

describe("filterAvailableVoiceModels", () => {
	const models = ["openai/Whisper Large", "vendor/speech-small", "whisper-turbo"];

	test("returns all models for an empty query", () => {
		expect(filterAvailableVoiceModels(models, "")).toEqual(models);
		expect(filterAvailableVoiceModels(models, "  ")).toEqual(models);
	});

	test("filters case-insensitively without rewriting IDs", () => {
		expect(filterAvailableVoiceModels(models, " WHISPER ")).toEqual(["openai/Whisper Large", "whisper-turbo"]);
	});

	test("handles missing or malformed model lists", () => {
		expect(filterAvailableVoiceModels(null, "speech")).toEqual([]);
		expect(filterAvailableVoiceModels(["", null, " speech "], "speech")).toEqual([" speech "]);
	});
});

function audioHarness(sampleRate = 16000) {
	const track = { stopped: false, stop() { this.stopped = true; } };
	const stream = { getTracks: () => [track] };
	const processor = {
		 onaudioprocess: null,
		connect() {},
		disconnect() { this.disconnected = true; },
	};
	const source = { connect() {}, disconnect() { this.disconnected = true; } };
	const context = {
		sampleRate,
		destination: {},
		createMediaStreamSource: () => source,
		createScriptProcessor: () => processor,
		close() { this.closed = true; return Promise.resolve(); },
	};
	return {
		track,
		processor,
		context,
		getUserMedia: async () => stream,
		audioContextFactory: () => context,
		push(samples) {
			processor.onaudioprocess?.({ inputBuffer: { getChannelData: () => Float32Array.from(samples) } });
		},
	};
}

function makeController(overrides = {}) {
	const audio = audioHarness();
	const states = [];
	let activeProfile = "artist";
	let draft = "existing draft";
	const inserted = [];
	const sent = [];
	const controller = createVoiceController({
		...audio,
		transcribe: async () => "recognized words",
		getDraft: () => draft,
		getProfileSlug: () => activeProfile,
		onInsert: (value) => { inserted.push(value); draft = value; },
		onSend: (value, profileSlug) => sent.push({ value, profileSlug }),
		onState: (value) => states.push(value),
		...overrides,
	});
	return { audio, controller, states, inserted, sent, getDraft: () => draft, setProfile(value) { activeProfile = value; } };
}

describe("voice audio encoding", () => {
	test("resamples mono Float32 audio to signed 16 kHz PCM", () => {
		expect(Array.from(resampleTo16k(new Float32Array([1, 1, -1, -1]), 32000))).toEqual([32767, -32768]);
	});

	test("writes a mono PCM16 WAV header at 16 kHz", () => {
		const wav = encodeWAV(new Int16Array([1, -1]));
		const view = new DataView(wav.buffer, wav.byteOffset, wav.byteLength);
		expect(new TextDecoder().decode(wav.subarray(0, 4))).toBe("RIFF");
		expect(new TextDecoder().decode(wav.subarray(8, 12))).toBe("WAVE");
		expect(view.getUint16(22, true)).toBe(1);
		expect(view.getUint32(24, true)).toBe(16000);
		expect(view.getUint16(34, true)).toBe(16);
		expect(view.getUint32(40, true)).toBe(4);
		expect(wav.byteLength).toBe(48);
	});
});

describe("createVoiceController", () => {
	test("cancel stops microphone tracks without transcribing or changing the draft", async () => {
		const testVoice = makeController();
		await testVoice.controller.start("artist");
		testVoice.controller.cancel();
		expect(testVoice.audio.track.stopped).toBe(true);
		expect(testVoice.inserted).toEqual([]);
		expect(testVoice.sent).toEqual([]);
		expect(testVoice.states.at(-1).phase).toBe("idle");
	});

	test("edit appends transcript with sensible spacing without submitting", async () => {
		const testVoice = makeController();
		await testVoice.controller.start("artist");
		testVoice.audio.push([0.25, -0.25]);
		await testVoice.controller.finish("edit");
		expect(testVoice.inserted).toEqual(["existing draft recognized words"]);
		expect(testVoice.sent).toEqual([]);
		expect(testVoice.audio.track.stopped).toBe(true);
	});

	test("send submits the combined draft once and ignores duplicate finish actions", async () => {
		let finishTranscription;
		let releaseSend;
		const pendingTranscription = new Promise((resolve) => { finishTranscription = resolve; });
		const pendingSend = new Promise((resolve) => { releaseSend = resolve; });
		let phaseWhenSendStarts = "";
		const testVoice = makeController({
			transcribe: () => pendingTranscription,
			onSend: (value, profileSlug) => {
				phaseWhenSendStarts = testVoice.states.at(-1).phase;
				testVoice.sent.push({ value, profileSlug });
				return pendingSend;
			},
		});
		await testVoice.controller.start("artist");
		testVoice.audio.push([0.1, -0.1]);
		const firstFinish = testVoice.controller.finish("send");
		await Promise.resolve();
		expect(await testVoice.controller.finish("send")).toBe(false);
		finishTranscription("once");
		await Bun.sleep(0);
		expect(testVoice.sent).toEqual([{ value: "existing draft once", profileSlug: "artist" }]);
		expect(phaseWhenSendStarts).toBe("idle");
		expect(testVoice.states.at(-1).phase).toBe("idle");
		releaseSend();
		await firstFinish;
	});

	test("edit callback starts after processing ends", async () => {
		let phaseWhenInsertStarts = "";
		const testVoice = makeController({
			onInsert: (value) => {
				phaseWhenInsertStarts = testVoice.states.at(-1).phase;
				testVoice.inserted.push(value);
			},
		});
		await testVoice.controller.start("artist");
		testVoice.audio.push([0.1, -0.1]);
		await testVoice.controller.finish("edit");
		expect(phaseWhenInsertStarts).toBe("idle");
	});

	test("resumes a suspended audio context before starting capture", async () => {
		const audio = audioHarness();
		audio.context.state = "suspended";
		let resumeContext;
		let resumeCalls = 0;
		audio.context.resume = () => {
			resumeCalls++;
			return new Promise((resolve) => {
				resumeContext = () => {
					audio.context.state = "running";
					resolve();
				};
			});
		};
		const states = [];
		const controller = createVoiceController({
			...audio,
			audioContextFactory: () => audio.context,
			transcribe: async () => "recognized words",
			onState: (state) => states.push(state),
		});

		const starting = controller.start("artist");
		await Bun.sleep(0);
		expect(resumeCalls).toBe(1);
		expect(states.at(-1).phase).toBe("requesting");

		resumeContext();
		expect(await starting).toBe(true);
		expect(states.at(-1).phase).toBe("recording");
	});

	test("zero-sample recordings do not upload an empty WAV", async () => {
		let transcriptionCalls = 0;
		const testVoice = makeController({
			transcribe: async () => { transcriptionCalls++; return "unexpected transcript"; },
		});
		await testVoice.controller.start("artist");
		await testVoice.controller.finish("send");

		expect(transcriptionCalls).toBe(0);
		expect(testVoice.sent).toEqual([]);
		expect(testVoice.inserted).toEqual([]);
		expect(testVoice.getDraft()).toBe("existing draft");
		expect(testVoice.audio.track.stopped).toBe(true);
		expect(testVoice.states.at(-1).phase).toBe("error");
		expect(testVoice.states.at(-1).error).toContain("No audio was captured");
	});

	test("empty and failed transcriptions preserve the draft and never submit", async () => {
		const empty = makeController({ transcribe: async () => "  " });
		await empty.controller.start("artist");
		empty.audio.push([0.1, -0.1]);
		await empty.controller.finish("send");
		expect(empty.inserted).toEqual([]);
		expect(empty.sent).toEqual([]);
		expect(empty.getDraft()).toBe("existing draft");
		expect(empty.audio.track.stopped).toBe(true);
		expect(empty.states.at(-1).phase).toBe("error");

		const failed = makeController({ transcribe: async () => { throw new Error("transcription unavailable"); } });
		await failed.controller.start("artist");
		failed.audio.push([0.1, -0.1]);
		await failed.controller.finish("edit");
		expect(failed.inserted).toEqual([]);
		expect(failed.sent).toEqual([]);
		expect(failed.getDraft()).toBe("existing draft");
		expect(failed.audio.track.stopped).toBe(true);
		expect(failed.states.at(-1).error).toContain("transcription unavailable");
	});

	test("a profile change during transcription prevents send", async () => {
		let finishTranscription;
		const pendingTranscription = new Promise((resolve) => { finishTranscription = resolve; });
		const testVoice = makeController({ transcribe: () => pendingTranscription });
		await testVoice.controller.start("artist");
		testVoice.audio.push([0.1, -0.1]);
		const finishing = testVoice.controller.finish("send");
		await Promise.resolve();
		testVoice.setProfile("");
		finishTranscription("private text");
		await finishing;
		expect(testVoice.sent).toEqual([]);
		expect(testVoice.inserted).toEqual([]);
		expect(testVoice.states.at(-1).error).toContain("profile changed");
	});

	test("permission failure and disposal release resources", async () => {
		const denied = makeController({ getUserMedia: async () => { throw new Error("permission denied"); } });
		await expect(denied.controller.start("artist")).rejects.toThrow("permission denied");
		expect(denied.states.at(-1).phase).toBe("error");

		const disposed = makeController();
		await disposed.controller.start("artist");
		await disposed.controller.dispose();
		expect(disposed.audio.track.stopped).toBe(true);
		expect(disposed.audio.context.closed).toBe(true);
	});

	test("elapsed timer reports duration and automatically finishes at the recording cap", async () => {
		let now = 0;
		let tick;
		let cleared = false;
		const testVoice = makeController({
			maxDurationMs: 1000,
			now: () => now,
			setIntervalFn: (callback) => { tick = callback; return 7; },
			clearIntervalFn: (id) => { cleared = id === 7; },
		});
		await testVoice.controller.start("artist");
		testVoice.audio.push([0.1, -0.1]);
		now = 600;
		tick();
		expect(testVoice.states.at(-1).elapsedMs).toBe(600);
		now = 1000;
		tick();
		await Bun.sleep(0);
		expect(testVoice.inserted).toEqual(["existing draft recognized words"]);
		expect(cleared).toBe(true);
	});
});
