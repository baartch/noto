import { readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { expect, test } from "@playwright/test";

const uiRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

test("Bun build has no esbuild dependency and emits a classic browser bundle", async () => {
	const packageJSON = JSON.parse(await readFile(path.join(uiRoot, "package.json"), "utf8"));
	expect(packageJSON.devDependencies?.esbuild).toBeUndefined();
	expect(packageJSON.dependencies?.esbuild).toBeUndefined();
	const bundle = await readFile(path.resolve(uiRoot, "../static/app.js"), "utf8");
	expect(bundle).toMatch(/^\(\(\)=>\{/);
	expect(bundle).not.toMatch(/^(?:import|export)\s/m);
});

test("workspace loads locally and mobile notes remain a separate tab", async ({ page, isMobile }) => {
	const failures = [];
	const externalRequests = [];
	const pageErrors = [];
	page.on("requestfailed", (request) => failures.push(`${request.url()}: ${request.failure()?.errorText}`));
	page.on("pageerror", (error) => pageErrors.push(error.message));
	page.on("response", (response) => {
		const url = new URL(response.url());
		if (url.origin === "http://127.0.0.1:4173" && !url.pathname.startsWith("/api/") && response.status() >= 400) {
			failures.push(`${response.status()} ${url.pathname}`);
		}
	});
	page.on("request", (request) => {
		if (new URL(request.url()).origin !== "http://127.0.0.1:4173") externalRequests.push(request.url());
	});
	await page.addInitScript(() => {
		window.__cspViolations = [];
		window.addEventListener("securitypolicyviolation", (event) => {
			window.__cspViolations.push(`${event.violatedDirective}: ${event.blockedURI}`);
		});
	});
	await page.goto("/");
	await expect(page.locator("#profile-name")).toHaveText("Artist");
	await expect(page.locator("#settings-button")).toBeVisible();
	await expect(page.locator("#message-input")).toBeVisible();
	await expect.poll(() => page.evaluate(() => window.$hsTabsCollection?.length || 0)).toBeGreaterThan(0);
	if (isMobile) {
		await expect(page.locator("#memory-panel")).toBeHidden();
		await page.locator("#mobile-memory-tab").click();
		await expect(page.locator("#memory-panel")).toBeVisible();
		await expect(page.locator("#mobile-memory-tab")).toHaveAttribute("aria-pressed", "true");
		await page.locator("#mobile-chat-tab").click();
		await expect(page.locator("#chat-panel")).toBeVisible();
		await expect(page.locator("#mobile-chat-tab")).toHaveAttribute("aria-pressed", "true");
	} else {
		await expect(page.locator("#memory-panel")).toBeVisible();
	}
	await expect.poll(() => failures.length).toBe(0);
	expect(externalRequests).toEqual([]);
	expect(pageErrors).toEqual([]);
	expect(await page.evaluate(() => window.__cspViolations)).toEqual([]);
});

test("gear opens the Settings dialog", async ({ page }) => {
	await page.goto("/");
	await page.locator("#settings-button").click();
	await expect(page.getByRole("dialog", { name: "Settings" })).toBeVisible({ timeout: 1000 });
});

const sampleConversation = [
	{ id: "assistant-1", role: "assistant", content: "A useful answer", content_html: "<p>A useful <strong>answer</strong></p>", created_at: "2026-11-22T10:00:00Z", conversation_id: "conversation-1" },
	{ id: "user-1", role: "user", content: "A follow-up", content_html: "<p>A follow-up</p>", created_at: "2026-11-22T10:01:00Z", conversation_id: "conversation-1" },
];

async function serveConversation(page, items = sampleConversation) {
	await page.route("**/api/messages?*", (route) => route.fulfill({
		status: 200,
		contentType: "application/json",
		body: JSON.stringify({ items, next_cursor: null }),
	}));
}

test("chat layout uses alternating Preline user and assistant bubbles", async ({ page }) => {
	await serveConversation(page);
	await page.goto("/");
	const assistant = page.getByRole("article", { name: "Noto response" });
	const user = page.getByRole("article", { name: "Your message" });
	await expect(assistant).toHaveClass(/gap-x-2/);
	await expect(assistant.locator("[data-message-bubble=assistant]")).toHaveClass(/bg-panel/);
	await expect(assistant.locator("[data-message-bubble=assistant]")).toHaveClass(/rounded-lg/);
	await expect(assistant.locator("[data-message-avatar=assistant]")).toBeVisible();
	await expect(user).toHaveClass(/justify-end/);
	await expect(user.locator("[data-message-bubble=user]")).toHaveClass(/bg-accent/);
	await expect(user.locator("[data-message-bubble=user]")).toHaveClass(/rounded-lg/);
	await expect(user.locator("[data-message-avatar=user]")).toBeVisible();
});

test("copy preserves rich HTML and plain text formats", async ({ page }) => {
	await page.context().grantPermissions(["clipboard-read", "clipboard-write"]);
	await serveConversation(page);
	await page.goto("/");
	const assistant = page.getByRole("article", { name: "Noto response" });
	const copy = assistant.getByRole("button", { name: "Copy message" });
	await copy.click();
	await expect(assistant.getByRole("button", { name: "Message copied" })).toBeVisible();
	const clipboard = await page.evaluate(async () => {
		const [item] = await navigator.clipboard.read();
		return {
			html: await (await item.getType("text/html")).text(),
			text: await (await item.getType("text/plain")).text(),
		};
	});
	expect(clipboard.html).toContain("<strong>answer</strong>");
	expect(clipboard.text).toContain("A useful answer");
});

test("optimistic message stays text until the Markdown preview returns", async ({ page }) => {
	await page.addInitScript(() => { window.__xssExecuted = 0; });
	let releaseMarkdown;
	let markdownStarted;
	const markdownRequestStarted = new Promise((resolve) => { markdownStarted = resolve; });
	let releaseSend;
	let sendStarted;
	const sendRequestStarted = new Promise((resolve) => { sendStarted = resolve; });
	await page.route("**/api/markdown", async (route) => {
		markdownStarted();
		await new Promise((resolve) => { releaseMarkdown = resolve; });
		await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ content_html: "<p>Sanitized preview</p>" }) });
	});
	await page.route("**/api/messages**", async (route) => {
		if (route.request().method() !== "POST") return route.continue();
		sendStarted();
		await new Promise((resolve) => { releaseSend = resolve; });
		await route.fulfill({ status: 200, contentType: "application/json", body: "{}" });
	});
	await page.goto("/");
	const payload = '<img src="x" onerror="window.__xssExecuted++">';
	await page.locator("#message-input").fill(payload);
	await page.locator("#message-form").evaluate((form) => form.requestSubmit());
	await Promise.all([markdownRequestStarted, sendRequestStarted]);
	const user = page.getByRole("article", { name: "Your message" });
	const content = user.locator("[data-message-content]");
	await expect(content).toHaveText(payload);
	await expect(user.locator("img")).toHaveCount(0);
	expect(await page.evaluate(() => window.__xssExecuted)).toBe(0);
	releaseMarkdown();
	await expect(content).toHaveText("Sanitized preview");
	await expect(user.locator("img")).toHaveCount(0);
	expect(await page.evaluate(() => window.__xssExecuted)).toBe(0);
	releaseSend();
});

test("long Markdown content does not widen the mobile viewport", async ({ page, isMobile }) => {
	test.skip(!isMobile, "mobile horizontal overflow regression");
	const longCell = "overflowcheck".repeat(48);
	await serveConversation(page, [{
		...sampleConversation[0],
		content_html: `<pre><code>${longCell}</code></pre><table><tbody><tr><td>${longCell}</td></tr></tbody></table>`,
	}]);
	await page.goto("/");
	await expect(page.locator("#message-list pre")).toBeVisible();
	await expect(page.locator("#message-list table")).toBeVisible();
	const dimensions = await page.evaluate(() => ({
		viewport: document.documentElement.clientWidth,
		scroll: document.documentElement.scrollWidth,
		pre: document.querySelector("pre") && {
			clientWidth: document.querySelector("pre").clientWidth,
			scrollWidth: document.querySelector("pre").scrollWidth,
			overflowX: getComputedStyle(document.querySelector("pre")).overflowX,
		},
		table: document.querySelector("table") && {
			clientWidth: document.querySelector("table").clientWidth,
			scrollWidth: document.querySelector("table").scrollWidth,
			overflowX: getComputedStyle(document.querySelector("table")).overflowX,
		},
	}));
	expect(dimensions.scroll).toBeLessThanOrEqual(dimensions.viewport);
	expect(dimensions.pre.overflowX).toMatch(/auto|scroll/);
	expect(dimensions.pre.scrollWidth).toBeGreaterThan(dimensions.pre.clientWidth);
	expect(dimensions.table.overflowX).toMatch(/auto|scroll/);
	expect(dimensions.table.scrollWidth).toBeGreaterThan(dimensions.table.clientWidth);
});

test("notes sidebar stays on the right and period tabs support keyboard navigation", async ({ page, isMobile }) => {
	const memoryRequests = [];
	await page.route("**/api/memory?*", async (route) => {
		const url = new URL(route.request().url());
		const tab = url.searchParams.get("tab");
		memoryRequests.push(tab);
		await route.fulfill({
			status: 200,
			contentType: "application/json",
			body: JSON.stringify({
				items: [{ id: `${tab}-note`, content: `${tab} note`, content_html: `<p>${tab} note</p>`, created_at: "2026-11-22T10:00:00Z", period_key: tab }],
				next_offset: null,
			}),
		});
	});
	await page.goto("/");
	const notes = page.locator("#memory-panel");
	if (isMobile) {
		await page.locator("#mobile-memory-tab").click();
		await expect(notes).toBeVisible();
	} else {
		const geometry = await page.evaluate(() => ({
			chatRight: document.querySelector("#chat-panel").getBoundingClientRect().right,
			notesLeft: document.querySelector("#memory-panel").getBoundingClientRect().left,
			chatOverflow: getComputedStyle(document.querySelector("#message-scroll")).overflowY,
			notesOverflow: getComputedStyle(document.querySelector("#memory-scroll")).overflowY,
		}));
		expect(geometry.notesLeft).toBeGreaterThanOrEqual(geometry.chatRight - 1);
		expect(geometry.chatOverflow).toMatch(/auto|scroll/);
		expect(geometry.notesOverflow).toMatch(/auto|scroll/);
	}
	const daily = notes.getByRole("tab", { name: "Daily" });
	const weekly = notes.getByRole("tab", { name: "Weekly" });
	await daily.focus();
	await daily.press("ArrowRight");
	await expect(weekly).toHaveAttribute("aria-selected", "true");
	await expect.poll(() => memoryRequests.at(-1)).toBe("weekly");
	await expect(notes.locator("#memory-list")).toContainText("weekly note");
});
