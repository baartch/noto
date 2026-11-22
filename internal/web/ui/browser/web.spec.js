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
