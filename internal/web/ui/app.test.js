import { expect, mock, test } from "bun:test";

mock.module("lucide", () => ({
	ArrowUp: {}, Bot: {}, ChevronDown: {}, Copy: {}, LoaderCircle: {}, MessagesSquare: {}, NotebookPen: {}, Sparkles: {},
	createIcons() {},
}));

class Element {
	constructor() {
		this.children = [];
		this.listeners = {};
		this.attributes = {};
		this.classes = new Set();
		this.classList = { toggle() {}, add: (name) => this.classes.add(name) };
		this.value = "";
		this.scrollHeight = 0;
		this.scrollTop = 0;
		this.clientHeight = 0;
	}
	append(...children) { this.children.push(...children); }
	replaceChildren(...children) { this.children = children; }
	setAttribute(name, value) { this.attributes[name] = value; }
	addEventListener(name, callback) { this.listeners[name] = callback; }
}

test("submission clears the input and formats the pending bubble before the reply", async () => {
	const elements = new Map();
	globalThis.document = {
		querySelector(selector) {
			if (!elements.has(selector)) elements.set(selector, new Element());
			return elements.get(selector);
		},
		querySelectorAll: () => [],
		createElement: () => new Element(),
		createTextNode: (text) => ({ textContent: text }),
	};
	globalThis.window = { sessionStorage: { getItem: () => null, setItem() {}, removeItem() {} }, matchMedia: () => ({ matches: true }), addEventListener() {} };
	let finishSend;
	const messageRequests = [];
	globalThis.fetch = async (url, options = {}) => {
		if (url === "/api/messages") {
			messageRequests.push(JSON.parse(options.body));
			return new Promise((resolve) => { finishSend = resolve; });
		}
		const body = url === "/api/state" ? { setup_required: false, profile: { slug: "artist" } } :
			url === "/api/markdown" ? { content_html: "<p>First</p>\n<p><strong>Second</strong></p>\n" } :
			{ items: [], next_cursor: null, next_offset: null };
		return { ok: true, json: async () => body };
	};
	await import("./app.js");
	await Bun.sleep(0);
	const input = elements.get("#message-input");
	input.value = "First\n\n**Second**";
	elements.get("#message-form").listeners.submit({ preventDefault() {} });
	expect(input.value).toBe("");
	expect(messageRequests[0]?.profile_slug).toBe("artist");
	let row = elements.get("#message-list").children.find((child) => child.attributes?.["aria-label"] === "Your message");
	expect(row.children[0].children[1].classes.has("whitespace-pre-wrap")).toBe(true);
	expect(row.children[0].children[1].textContent).toBe("First\n\n**Second**");
	await Bun.sleep(0);
	row = elements.get("#message-list").children.find((child) => child.attributes?.["aria-label"] === "Your message");
	expect(row.children[0].children[1].innerHTML).toBe("<p>First</p>\n<p><strong>Second</strong></p>\n");
	input.value = "new draft";
	finishSend({ ok: false, status: 502, json: async () => ({ error: { message: "failed" } }) });
	await Bun.sleep(0);
	expect(input.value).toBe("new draft");

	elements.get("#message-form").listeners.submit({ preventDefault() {} });
	expect(input.value).toBe("");
	finishSend({ ok: false, status: 502, json: async () => ({ error: { message: "failed" } }) });
	await Bun.sleep(0);
	expect(input.value).toBe("new draft");

	elements.get("#message-form").listeners.submit({ preventDefault() {} });
	input.value = "new draft";
	finishSend({ ok: true, json: async () => ({}) });
	await Bun.sleep(0);
	expect(input.value).toBe("new draft");
});
