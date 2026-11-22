import { describe, expect, test } from "bun:test";
import { filterAvailableVoiceModels, voiceReady } from "./voice.js";

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
