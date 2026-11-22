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

function securityHeaders() {
	return {
		"Content-Security-Policy": "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'; object-src 'none'",
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options": "DENY",
		"Referrer-Policy": "no-referrer",
		"Permissions-Policy": "camera=(), microphone=(), geolocation=()",
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
