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

test("chat, notes, and settings use only local assets under the production CSP", async ({ page, isMobile }) => {
	const externalRequests = [];
	const localPaths = [];
	await page.addInitScript(() => {
		window.__interactionCspViolations = [];
		window.addEventListener("securitypolicyviolation", (event) => window.__interactionCspViolations.push(`${event.violatedDirective}: ${event.blockedURI}`));
	});
	page.on("request", (request) => {
		const url = new URL(request.url());
		if (url.origin !== "http://127.0.0.1:4173") externalRequests.push(request.url());
		else localPaths.push(url.pathname);
	});
	await page.route("**/api/messages**", (route) => route.request().method() === "POST"
		? route.fulfill({ status: 200, contentType: "application/json", body: "{}" })
		: route.continue());
	await page.goto("/");
	const sendRequest = page.waitForRequest((request) => new URL(request.url()).pathname === "/api/messages" && request.method() === "POST");
	await page.locator("#message-input").fill("local web runtime");
	await page.locator("#message-form").evaluate((form) => form.requestSubmit());
	await sendRequest;
	if (isMobile) await page.locator("#mobile-memory-tab").click();
	await page.locator('[data-memory-tab="weekly"]').click();
	await expect(page.locator("#memory-tab-weekly")).toHaveAttribute("aria-selected", "true");
	await page.locator("#settings-button").click();
	const dialog = page.getByRole("dialog", { name: "Settings" });
	await expect(dialog).toBeVisible();
	await dialog.getByRole("tab", { name: "Models" }).click();
	const modelSearch = dialog.locator("#settings-model-search");
	await expect(modelSearch).toBeEnabled();
	await modelSearch.fill("model");
	await modelSearch.press("ArrowDown");
	await expect(modelSearch).toHaveAttribute("aria-activedescendant", /settings-model-options-option/);
	await dialog.getByRole("button", { name: "Cancel" }).click();
	await expect(dialog).toBeHidden();
	expect(localPaths).toContain("/api/messages");
	expect(localPaths).toContain("/api/settings");
	expect(localPaths).toContain("/api/settings/models");
	expect(localPaths).toContain("/api/memory");
	expect(externalRequests).toEqual([]);
	expect(await page.evaluate(() => window.__interactionCspViolations)).toEqual([]);
});

test("@settings gear opens a Preline modal with keyboard tabs and focus restoration", async ({ page, isMobile }) => {
	const violations = [];
	await page.addInitScript(() => {
		window.__settingsCspViolations = [];
		window.addEventListener("securitypolicyviolation", (event) => window.__settingsCspViolations.push(event.violatedDirective));
	});
	await page.goto("/");
	const trigger = page.locator("#settings-button");
	await trigger.click();
	const dialog = page.getByRole("dialog", { name: "Settings" });
	await expect(dialog).toBeVisible();
	await expect(dialog).toHaveAttribute("aria-modal", "true");
	await expect(page.locator("#chat-panel")).toBeVisible();
	await expect(dialog.locator("#settings-tabs")).toHaveAttribute("aria-orientation", isMobile ? "horizontal" : "vertical");
	const profileTab = dialog.getByRole("tab", { name: "Profile" });
	const modelsTab = dialog.getByRole("tab", { name: "Models" });
	await profileTab.focus();
	await profileTab.press(isMobile ? "ArrowRight" : "ArrowDown");
	await expect(modelsTab).toHaveAttribute("aria-selected", "true");
	await expect(dialog.locator("#settings-models-panel")).toBeVisible();
	await expect(dialog.locator("#settings-profile-panel")).toBeHidden();
	await profileTab.click();
	await expect(dialog.locator("#settings-profile-panel")).toBeVisible();
	for (let index = 0; index < 12; index++) {
		await page.keyboard.press("Tab");
		expect(await dialog.evaluate((element) => element.contains(document.activeElement)), `Tab ${index + 1} should stay in the dialog`).toBe(true);
	}
	for (let index = 0; index < 6; index++) {
		await page.keyboard.press("Shift+Tab");
		expect(await dialog.evaluate((element) => element.contains(document.activeElement))).toBe(true);
	}
	await modelsTab.click();
	const modelSearch = dialog.locator("#settings-model-search");
	await modelSearch.fill("model");
	await modelSearch.press("ArrowDown");
	await expect(modelSearch).toHaveAttribute("aria-activedescendant", /settings-model-options-option/);
	await page.keyboard.press("Escape");
	await expect(dialog).toBeVisible();
	await expect(modelSearch).toHaveAttribute("aria-expanded", "false");
	await page.keyboard.press("Escape");
	await expect(dialog).toBeHidden();
	await expect(trigger).toBeFocused();
	violations.push(...await page.evaluate(() => window.__settingsCspViolations));
	expect(violations).toEqual([]);
});

test("@settings cancel and backdrop discard without saving", async ({ page }) => {
	let settingsPosts = 0;
	page.on("request", (request) => {
		if (request.url().endsWith("/api/settings") && request.method() === "POST") settingsPosts++;
	});
	await page.goto("/");
	await page.locator("#settings-button").click();
	const dialog = page.getByRole("dialog", { name: "Settings" });
	await expect(dialog).toBeVisible();
	await page.locator("#settings-models-tab").click();
	await page.locator("#settings-panel-backdrop").click({ position: { x: 2, y: 2 } });
	await expect(dialog).toBeVisible();
	await dialog.getByRole("button", { name: "Cancel" }).click();
	await expect(dialog).toBeHidden();
	expect(settingsPosts).toBe(0);
	await expect(page.locator("#settings-button")).toBeFocused();
});

test("@settings save cannot be dismissed while the request is pending", async ({ page }) => {
	let releaseSave;
	let saveStarted;
	let saveRequests = 0;
	const saveRequestStarted = new Promise((resolve) => { saveStarted = resolve; });
	await page.route("**/api/settings", async (route) => {
		if (route.request().method() !== "POST") return route.continue();
		saveRequests++;
		saveStarted();
		await new Promise((resolve) => { releaseSave = resolve; });
		await route.fulfill({
			status: 200,
			contentType: "application/json",
			body: JSON.stringify({ profile: { slug: "artist", name: "Artist" }, model: "model-b", setup_required: false, revision: 2, sending: false, pending_notes: 0 }),
		});
	});
	await page.goto("/");
	await page.locator("#settings-button").click();
	const dialog = page.getByRole("dialog", { name: "Settings" });
	await expect(dialog).toBeVisible();
	await page.locator("#settings-models-tab").click();
	const modelSearch = page.locator("#settings-model-search");
	await modelSearch.fill("model-b");
	await modelSearch.press("ArrowDown");
	await modelSearch.press("Enter");
	await page.locator("#settings-save").click();
	await saveRequestStarted;
	await expect(page.locator("#settings-close")).toBeDisabled();
	await expect(page.locator("#settings-cancel")).toBeDisabled();
	await page.locator("#settings-close").click({ force: true });
	await page.locator("#settings-save").click({ force: true });
	await page.keyboard.press("Escape");
	await expect(dialog).toBeVisible();
	await page.locator("#settings-panel-backdrop").click({ position: { x: 2, y: 2 } });
	await expect(dialog).toBeVisible();
	await page.locator("#settings-cancel").click({ force: true });
	await expect(dialog).toBeVisible();
	expect(saveRequests).toBe(1);
	releaseSave();
	await expect(dialog).toBeHidden();
});

test("@settings model labels fit inside the responsive dialog", async ({ page }) => {
	const longModel = "provider/model-with-a-long-identifying-name-that-must-wrap-without-widening-the-dialog";
	await page.route("**/api/settings/models?*", (route) => route.fulfill({
		status: 200,
		contentType: "application/json",
		body: JSON.stringify({ profile_slug: "artist", saved_model: longModel, models: [longModel, "model-b"], saved_embeddings_model: "embed-a", embedding_models: ["embed-a"], saved_extractor_model: "extractor-a", setup_required: false }),
	}));
	await page.goto("/");
	await page.locator("#settings-button").click();
	const dialog = page.getByRole("dialog", { name: "Settings" });
	await expect(dialog).toBeVisible();
	await dialog.getByRole("tab", { name: "Models" }).click();
	const dimensions = await page.evaluate(() => ({
		viewport: document.documentElement.clientWidth,
		scroll: document.documentElement.scrollWidth,
		save: document.querySelector("#settings-save").getBoundingClientRect(),
	}));
	expect(dimensions.scroll).toBeLessThanOrEqual(dimensions.viewport);
	expect(dimensions.save.right).toBeLessThanOrEqual(dimensions.viewport);
	expect(dimensions.save.width).toBeGreaterThan(0);
});

test("a long active model name cannot push Settings off the compact desktop header", async ({ page, isMobile }) => {
	test.skip(isMobile, "compact desktop header regression");
	const longModel = `provider/${"model".repeat(36)}`;
	await page.setViewportSize({ width: 640, height: 900 });
	await page.route("**/api/state", (route) => route.fulfill({
		status: 200,
		contentType: "application/json",
		body: JSON.stringify({ profile: { slug: "artist", name: "Artist" }, model: longModel, setup_required: false, revision: 1, sending: false, pending_notes: 0 }),
	}));
	await page.goto("/");
	await expect(page.locator("#model-name")).toHaveText(longModel);
	const header = await page.evaluate(() => ({
		viewport: document.documentElement.clientWidth,
		scroll: document.documentElement.scrollWidth,
		gear: document.querySelector("#settings-button").getBoundingClientRect(),
		model: document.querySelector("#model-name"),
		modelStyle: getComputedStyle(document.querySelector("#model-name")),
	}));
	expect(header.gear.right).toBeLessThanOrEqual(header.viewport);
	expect(header.scroll).toBeLessThanOrEqual(header.viewport);
	expect(header.modelStyle.textOverflow).toBe("ellipsis");
	expect(await page.locator("#model-name").getAttribute("title")).toBe(longModel);
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

test("message metadata and copy control sit below and align with each bubble", async ({ page }) => {
	await serveConversation(page);
	await page.goto("/");
	for (const [role, expectedName, align] of [["assistant", "Noto", "left"], ["user", "You", "right"]]) {
		const message = page.locator(`[data-message-bubble=${role}]`).locator("xpath=ancestor::article");
		const bubble = message.locator(`[data-message-bubble=${role}]`);
		const footer = message.locator("[data-message-footer]");
		const meta = message.locator("[data-message-meta]");
		const copy = message.getByRole("button", { name: "Copy message" });
		await expect(meta).toContainText(expectedName);
		await expect(meta.locator("time")).toBeVisible();
		await expect(meta.locator("xpath=ancestor::*[@data-message-bubble]")).toHaveCount(0);
		const [bubbleBox, footerBox, copyBox, footerParent, metaParent, copyParent] = await Promise.all([
			bubble.boundingBox(),
			footer.boundingBox(),
			copy.boundingBox(),
			footer.evaluate((element) => element.parentElement),
			meta.evaluate((element) => element.parentElement),
			copy.evaluate((element) => element.parentElement),
		]);
		expect(footerBox.y).toBeGreaterThanOrEqual(bubbleBox.y + bubbleBox.height);
		expect(metaParent).toBe(footerParent);
		expect(copyParent).toBe(metaParent);
		expect(copyBox.width).toBeLessThanOrEqual(16);
		expect(copyBox.height).toBeLessThanOrEqual(16);
		if (align === "left") expect(footerBox.x).toBeCloseTo(bubbleBox.x, 0);
		else expect(footerBox.x + footerBox.width).toBeCloseTo(bubbleBox.x + bubbleBox.width, 0);
	}
});

test("dark app palette matches Preline surfaces and primary colors", async ({ page }) => {
	await serveConversation(page, [
		{ ...sampleConversation[0], content_html: '<p><a href="/docs">Reference link</a></p>' },
		sampleConversation[1],
	]);
	await page.goto("/");
	await expect(page.locator("[data-message-bubble=assistant]")).toBeVisible();
	await expect(page.locator("[data-message-bubble=user]")).toBeVisible();
	const chatColors = await page.evaluate(() => ({
		themeColor: document.querySelector('meta[name="theme-color"]').content,
		page: getComputedStyle(document.body).backgroundColor,
		header: getComputedStyle(document.querySelector("header")).backgroundColor,
		notes: getComputedStyle(document.querySelector("#memory-panel")).backgroundColor,
		assistant: getComputedStyle(document.querySelector("[data-message-bubble=assistant]")).backgroundColor,
		user: getComputedStyle(document.querySelector("[data-message-bubble=user]")).backgroundColor,
		composer: getComputedStyle(document.querySelector("#message-form")).backgroundColor,
		composerBorder: getComputedStyle(document.querySelector("#message-form")).borderTopColor,
		link: getComputedStyle(document.querySelector(".markdown-body a")).color,
		muted: getComputedStyle(document.querySelector("#model-name")).color,
	}));
	expect(chatColors.themeColor).toBe("#171717");
	expect(chatColors.page).toBe("rgb(23, 23, 23)");
	expect(chatColors.header).toBe("rgb(38, 38, 38)");
	expect(chatColors.notes).toBe("rgb(38, 38, 38)");
	expect(chatColors.assistant).toBe("rgb(38, 38, 38)");
	expect(chatColors.user).toBe("rgb(59, 130, 246)");
	expect(chatColors.composer).toBe("rgb(38, 38, 38)");
	expect(chatColors.composerBorder).toBe("rgb(64, 64, 64)");
	expect(chatColors.link).toBe("rgb(96, 165, 250)");
	expect(chatColors.muted).toBe("rgb(163, 163, 163)");
	await page.locator("#settings-button").click();
	const settingsColors = await page.evaluate(() => ({
		panel: getComputedStyle(document.querySelector("#settings-panel > div > section")).backgroundColor,
		input: getComputedStyle(document.querySelector("#settings-profile-search")).backgroundColor,
		primary: getComputedStyle(document.querySelector("#settings-save")).backgroundColor,
	}));
	expect(settingsColors.panel).toBe("rgb(38, 38, 38)");
	expect(settingsColors.input).toBe("rgb(64, 64, 64)");
	expect(settingsColors.primary).toBe("rgb(59, 130, 246)");
});

test("copy preserves rich HTML and plain text formats", async ({ page }) => {
	await page.context().grantPermissions(["clipboard-read", "clipboard-write"]);
	await serveConversation(page);
	await page.goto("/");
	const assistant = page.getByRole("article", { name: "Noto response" });
	const copy = assistant.getByRole("button", { name: "Copy message" });
	await expect(copy).toHaveText("");
	const copyBox = await copy.boundingBox();
	expect(copyBox.width).toBeLessThanOrEqual(36);
	expect(copyBox.height).toBeLessThanOrEqual(36);
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
