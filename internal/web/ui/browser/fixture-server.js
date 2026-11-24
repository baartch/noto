import { extname, resolve, sep } from "node:path";

const staticRoot = resolve(import.meta.dir, "../../static");
const profile = { slug: "artist", name: "Artist" };
const profileModels = {
	profile_slug: "artist",
	saved_model: "model-a",
	models: ["model-a", "model-b"],
	models_unavailable: false,
	saved_embeddings_model: "embed-a",
	embedding_models: ["embed-a", "embed-b"],
	embedding_models_unavailable: false,
	saved_extractor_model: "extractor-a",
	setup_required: false,
};
let voiceSelection = {
	profile_slug: profile.slug,
	provider: "",
	model: "",
	ready: false,
	model_status: { available: true, installed: false, downloading: false, deleting: false, downloaded_bytes: 0, download_size_bytes: 487211443, installed_size_bytes: 670576784, progress: 0 },
	reason: "Choose a voice backend in Settings.",
	revision: 1,
};
let voiceModels = ["openrouter/whisper-large", "vendor/stt-small"];
let localModelStatus = voiceSelection.model_status;
let localModelPolls = 0;

function currentVoiceSelection(profileSlug) {
	const localReady = voiceSelection.provider === "local"
		&& voiceSelection.model === "parakeet-tdt-0.6b-v3-int8"
		&& localModelStatus.available
		&& localModelStatus.installed;
	const providerReady = voiceSelection.provider === "provider"
		&& Boolean(voiceSelection.model)
		&& voiceModels.includes(voiceSelection.model);
	const ready = localReady || providerReady;
	return {
		...voiceSelection,
		profile_slug: profileSlug,
		ready,
		model_status: { ...localModelStatus },
		reason: ready ? "" : voiceSelection.provider === "local" ? "Download the local speech model to enable dictation." : "Choose a voice backend in Settings.",
	};
}

function securityHeaders() {
	return {
		"Content-Security-Policy": "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'; object-src 'none'",
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options": "DENY",
		"Referrer-Policy": "no-referrer",
		"Permissions-Policy": "camera=(), microphone=(self), geolocation=()",
		"Cross-Origin-Resource-Policy": "same-origin",
	};
}

function json(value) {
	return Response.json(value, { headers: securityHeaders() });
}

Bun.serve({
	hostname: "127.0.0.1",
	port: 4173,
	async fetch(request) {
		const url = new URL(request.url);
		if (url.pathname === "/healthz") return new Response("ok");
		if (url.pathname === "/api/state") {
			return json({ profile, model: profileModels.saved_model, setup_required: false, revision: 1, sending: false, pending_notes: 0 });
		}
		if (url.pathname.startsWith("/api/messages")) return json({ items: [], next_cursor: null });
		if (url.pathname.startsWith("/api/memory")) return json({ items: [], next_offset: null });
		if (url.pathname === "/api/settings") return json({ active_profile_slug: profile.slug, profiles: [profile] });
		if (url.pathname.startsWith("/api/settings/models")) return json(profileModels);
		if (url.pathname === "/api/voice") {
			if (request.method === "POST") {
				voiceSelection = { ...voiceSelection, ...(await request.json()), revision: voiceSelection.revision + 1 };
				return json(currentVoiceSelection(voiceSelection.profile_slug));
			}
			return json(currentVoiceSelection(url.searchParams.get("profile_slug") || profile.slug));
		}
		if (url.pathname === "/api/voice/models") return json({ profile_slug: url.searchParams.get("profile_slug") || profile.slug, models: voiceModels, revision: voiceSelection.revision });
		if (url.pathname === "/api/voice/local-model") {
			if (localModelStatus.downloading) {
				localModelPolls++;
				localModelStatus = localModelPolls > 2
					? { ...localModelStatus, installed: true, downloading: false, downloaded_bytes: localModelStatus.download_size_bytes, progress: 1 }
					: { ...localModelStatus, downloaded_bytes: Math.round(localModelStatus.download_size_bytes * localModelPolls / 4), progress: localModelPolls / 4 };
			}
			return json({ model_status: { ...localModelStatus } });
		}
		if (url.pathname === "/api/voice/local-model/download" && request.method === "POST") {
			localModelPolls = 0;
			localModelStatus = { ...localModelStatus, downloading: true, progress: 0 };
			return Response.json({ model_status: { ...localModelStatus } }, { status: 202, headers: securityHeaders() });
		}
		if (url.pathname === "/api/voice/local-model" && request.method === "DELETE") {
			localModelStatus = { ...localModelStatus, installed: false, downloading: false, downloaded_bytes: 0, progress: 0 };
			return json({ model_status: { ...localModelStatus } });
		}
		if (url.pathname === "/api/voice/transcriptions" && request.method === "POST") return json({ text: "fixture transcript" });
		if (url.pathname === "/api/markdown") return json({ content_html: "" });
		if (url.pathname.startsWith("/api/")) return json({ error: { code: "not_found", message: "Not found" } });

		const asset = resolve(staticRoot, `.${decodeURIComponent(url.pathname === "/" ? "/index.html" : url.pathname)}`);
		if (asset !== staticRoot && !asset.startsWith(`${staticRoot}${sep}`)) return new Response("Not found", { status: 404, headers: securityHeaders() });
		const file = Bun.file(asset);
		if (!(await file.exists())) return new Response("Not found", { status: 404, headers: securityHeaders() });
		const contentType = new Map([[".html", "text/html; charset=utf-8"], [".js", "text/javascript; charset=utf-8"], [".css", "text/css; charset=utf-8"]]).get(extname(asset)) || "application/octet-stream";
		return new Response(file, { headers: { ...securityHeaders(), "Content-Type": contentType, "Cache-Control": "no-cache" } });
	},
});
