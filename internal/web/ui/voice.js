export const LOCAL_VOICE_MODEL_ID = "parakeet-tdt-0.6b-v3-int8";

export function voiceReady(voiceDTO) {
	if (!voiceDTO || voiceDTO.ready !== true) return false;
	const provider = voiceDTO.provider;
	const model = typeof voiceDTO.model === "string" ? voiceDTO.model.trim() : "";
	if (!model) return false;
	if (provider === "local") {
		return model === LOCAL_VOICE_MODEL_ID
			&& voiceDTO.model_status?.available === true
			&& voiceDTO.model_status?.installed === true;
	}
	return provider === "provider";
}

export function filterAvailableVoiceModels(models, search) {
	if (!Array.isArray(models)) return [];
	const query = String(search ?? "").trim().toLocaleLowerCase();
	return models.filter((model) => typeof model === "string"
		&& model.trim() !== ""
		&& (!query || model.toLocaleLowerCase().includes(query)));
}

export const VOICE_SAMPLE_RATE = 16000;
export const VOICE_MAX_DURATION_MS = 5 * 60 * 1000;

export function resampleTo16k(samples, inputSampleRate) {
	if (!samples || typeof samples.length !== "number" || !Number.isFinite(inputSampleRate) || inputSampleRate <= 0) {
		throw new TypeError("audio samples and a valid sample rate are required");
	}
	const ratio = inputSampleRate / VOICE_SAMPLE_RATE;
	const outputLength = Math.floor(samples.length / ratio);
	const output = new Int16Array(outputLength);
	for (let index = 0; index < outputLength; index++) {
		let sample;
		if (ratio > 1) {
			const start = index * ratio;
			const end = Math.min(samples.length, (index + 1) * ratio);
			let weighted = 0;
			let weightTotal = 0;
			for (let source = Math.floor(start); source < end; source++) {
				const weight = Math.min(source + 1, end) - Math.max(source, start);
				weighted += samples[source] * weight;
				weightTotal += weight;
			}
			sample = weightTotal ? weighted / weightTotal : 0;
		} else {
			const position = index * ratio;
			const left = Math.floor(position);
			const blend = position - left;
			sample = samples[left] * (1 - blend) + (samples[Math.min(left + 1, samples.length - 1)] || 0) * blend;
		}
		const clipped = Math.max(-1, Math.min(1, sample));
		output[index] = clipped < 0 ? Math.round(clipped * 32768) : Math.round(clipped * 32767);
	}
	return output;
}

export function encodeWAV(pcmSamples, sampleRate = VOICE_SAMPLE_RATE) {
	if (!(pcmSamples instanceof Int16Array) || !Number.isInteger(sampleRate) || sampleRate <= 0) {
		throw new TypeError("WAV encoding requires Int16 PCM samples and a valid sample rate");
	}
	const bytes = new Uint8Array(44 + pcmSamples.length * 2);
	const view = new DataView(bytes.buffer);
	const writeText = (offset, text) => {
		for (let index = 0; index < text.length; index++) bytes[offset + index] = text.charCodeAt(index);
	};
	writeText(0, "RIFF");
	view.setUint32(4, bytes.byteLength - 8, true);
	writeText(8, "WAVE");
	writeText(12, "fmt ");
	view.setUint32(16, 16, true);
	view.setUint16(20, 1, true);
	view.setUint16(22, 1, true);
	view.setUint32(24, sampleRate, true);
	view.setUint32(28, sampleRate * 2, true);
	view.setUint16(32, 2, true);
	view.setUint16(34, 16, true);
	writeText(36, "data");
	view.setUint32(40, pcmSamples.length * 2, true);
	for (let index = 0; index < pcmSamples.length; index++) view.setInt16(44 + index * 2, pcmSamples[index], true);
	return bytes;
}

export function appendVoiceTranscript(draft, transcript) {
	const existing = typeof draft === "string" ? draft : "";
	const spoken = typeof transcript === "string" ? transcript.trim() : "";
	if (!spoken) return existing;
	if (!existing.trim()) return spoken;
	return `${existing}${/\s$/u.test(existing) ? "" : " "}${spoken}`;
}

export function createVoiceController({
	getUserMedia = (constraints) => navigator.mediaDevices.getUserMedia(constraints),
	audioContextFactory = () => new (window.AudioContext || window.webkitAudioContext)(),
	transcribe,
	getDraft = () => "",
	getProfileSlug = null,
	onInsert = () => {},
	onSend = () => {},
	onState = () => {},
	now = () => Date.now(),
	setIntervalFn = (callback, delay) => globalThis.setInterval(callback, delay),
	clearIntervalFn = (id) => globalThis.clearInterval(id),
	maxDurationMs = VOICE_MAX_DURATION_MS,
}) {
	let state = { phase: "idle", elapsedMs: 0, volume: 0, error: "", profileSlug: "" };
	let disposed = false;
	let generation = 0;
	let activeProfileSlug = "";
	let stream = null;
	let audioContext = null;
	let source = null;
	let processor = null;
	let intervalID = null;
	let startedAt = 0;
	let chunks = [];
	let sampleCount = 0;
	let transcribeController = null;

	const publish = (patch) => {
		state = { ...state, ...patch };
		onState({ ...state });
	};
	const stopTracks = (target) => {
		for (const track of target?.getTracks?.() || []) {
			try { track.stop(); } catch { /* Track shutdown is best-effort. */ }
		}
	};
	const stopTimer = () => {
		if (intervalID !== null) clearIntervalFn(intervalID);
		intervalID = null;
	};
	const releaseAudio = async () => {
		stopTimer();
		if (processor) {
			processor.onaudioprocess = null;
			try { processor.disconnect(); } catch { /* Audio graph may already be stopped. */ }
		}
		try { source?.disconnect(); } catch { /* Audio graph may already be stopped. */ }
		stopTracks(stream);
		stream = null;
		processor = null;
		source = null;
		const closing = audioContext;
		audioContext = null;
		if (closing && closing.state !== "closed") {
			try { await closing.close(); } catch { /* Context shutdown is best-effort. */ }
		}
	};
	const finishAtLimit = async () => {
		const elapsedMs = Math.min(maxDurationMs, Math.max(0, now() - startedAt));
		publish({ elapsedMs });
		if (elapsedMs >= maxDurationMs && state.phase === "recording") await controller.finish("edit");
	};
	const resetCapture = () => {
		chunks = [];
		sampleCount = 0;
		startedAt = 0;
	};

	const controller = {
		async start(profileSlug) {
			if (disposed || !profileSlug || !["idle", "error"].includes(state.phase)) return false;
			const requestGeneration = ++generation;
			activeProfileSlug = profileSlug;
			resetCapture();
			publish({ phase: "requesting", elapsedMs: 0, volume: 0, error: "", profileSlug });
			try {
				const nextStream = await getUserMedia({ audio: { channelCount: 1, echoCancellation: true, noiseSuppression: true, autoGainControl: true }, video: false });
				if (disposed || requestGeneration !== generation) {
					stopTracks(nextStream);
					return false;
				}
				stream = nextStream;
				audioContext = await audioContextFactory();
				if (disposed || requestGeneration !== generation) {
					await releaseAudio();
					return false;
				}
				if (!audioContext || typeof audioContext.createScriptProcessor !== "function" || typeof audioContext.createMediaStreamSource !== "function") {
					throw new Error("This browser does not support microphone audio processing.");
				}
				if (audioContext.state === "suspended") {
					await audioContext.resume();
					if (disposed || requestGeneration !== generation) {
						await releaseAudio();
						return false;
					}
				}
				source = audioContext.createMediaStreamSource(stream);
				processor = audioContext.createScriptProcessor(4096, 1, 1);
				const maxSamples = Math.floor(maxDurationMs * VOICE_SAMPLE_RATE / 1000);
				processor.onaudioprocess = (event) => {
					if (state.phase !== "recording") return;
					const input = event.inputBuffer.getChannelData(0);
					const remaining = Math.max(0, maxSamples - sampleCount);
					if (remaining > 0) {
						const converted = resampleTo16k(input, audioContext.sampleRate);
						const kept = converted.length > remaining ? converted.subarray(0, remaining) : converted;
						if (kept.length) {
							chunks.push(kept);
							sampleCount += kept.length;
						}
					}
					let squareTotal = 0;
					for (const sample of input) squareTotal += sample * sample;
					const volume = input.length ? Math.min(1, Math.sqrt(squareTotal / input.length) * 4) : 0;
					publish({ volume });
					if (sampleCount >= maxSamples) void controller.finish("edit");
				};
				source.connect(processor);
				processor.connect(audioContext.destination);
				startedAt = now();
				publish({ phase: "recording", elapsedMs: 0, volume: 0 });
				intervalID = setIntervalFn(() => { void finishAtLimit(); }, 100);
				return true;
			} catch (error) {
				if (requestGeneration === generation && !disposed) {
					await releaseAudio();
					resetCapture();
					publish({ phase: "error", volume: 0, error: error?.message || "Microphone access failed." });
				}
				throw error;
			}
		},
		async finish(action) {
			if (!["edit", "send"].includes(action) || state.phase !== "recording" || disposed) return false;
			const finishGeneration = generation;
			const profileSlug = activeProfileSlug;
			publish({ phase: "processing", error: "", volume: 0, elapsedMs: Math.min(maxDurationMs, Math.max(0, now() - startedAt)) });
			const pcm = new Int16Array(sampleCount);
			let offset = 0;
			for (const chunk of chunks) {
				pcm.set(chunk.subarray(0, Math.max(0, sampleCount - offset)), offset);
				offset += chunk.length;
			}
			chunks = [];
			await releaseAudio();
			const profileIsCurrent = () => typeof getProfileSlug !== "function" || getProfileSlug() === profileSlug;
			if (disposed || finishGeneration !== generation) return false;
			if (!profileIsCurrent()) {
				publish({ phase: "error", error: "The active profile changed. The recording was discarded." });
				return false;
			}
			if (pcm.length === 0) {
				resetCapture();
				publish({ phase: "error", error: "No audio was captured. Please try again." });
				return false;
			}
			transcribeController = new AbortController();
			try {
				if (typeof transcribe !== "function") throw new Error("Voice transcription is unavailable.");
				const result = await transcribe(encodeWAV(pcm), profileSlug, transcribeController.signal);
				if (disposed || finishGeneration !== generation) return false;
				if (!profileIsCurrent()) {
					publish({ phase: "error", error: "The active profile changed. The recording was discarded." });
					return false;
				}
				const transcript = typeof result === "string" ? result : result?.text;
				if (typeof transcript !== "string" || !transcript.trim()) {
					publish({ phase: "error", error: "No speech was detected. Your draft was kept." });
					return false;
				}
				const combined = appendVoiceTranscript(getDraft(), transcript);
				publish({ phase: "idle", error: "", volume: 0 });
				if (action === "edit") await onInsert(combined, profileSlug);
				else await onSend(combined, profileSlug);
				return !disposed && finishGeneration === generation;
			} catch (error) {
				if (!disposed && finishGeneration === generation) {
					publish({ phase: "error", error: error?.message || "Transcription failed." });
				}
				return false;
			} finally {
				transcribeController = null;
				resetCapture();
			}
		},
		cancel() {
			if (disposed) return;
			generation++;
			transcribeController?.abort();
			transcribeController = null;
			void releaseAudio();
			resetCapture();
			publish({ phase: "idle", elapsedMs: 0, volume: 0, error: "", profileSlug: "" });
		},
		async dispose() {
			if (disposed) return;
			disposed = true;
			generation++;
			transcribeController?.abort();
			transcribeController = null;
			await releaseAudio();
			resetCapture();
		},
	};
	return controller;
}
