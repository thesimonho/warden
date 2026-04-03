import { defineConfig } from "astro/config";
import starlight from "@astrojs/starlight";
import starlightOpenAPI, { openAPISidebarGroups } from "starlight-openapi";
import starlightLlmsTxt from "starlight-llms-txt";
import starlightScrollToTop from "starlight-scroll-to-top";
export default defineConfig({
	site: "https://thesimonho.github.io",
	base: "/warden",
	integrations: [
		starlight({
			title: "Warden",
			logo: {
				light: "./src/assets/logo-light.svg",
				dark: "./src/assets/logo-dark.svg",
				replacesTitle: true,
			},
			description:
				"A modular security boundary for AI coding agents. Container isolation, worktree orchestration, and agent monitoring.",
			social: [
				{
					icon: "github",
					label: "GitHub",
					href: "https://github.com/thesimonho/warden",
				},
			],
			editLink: {
				baseUrl: "https://github.com/thesimonho/warden/edit/main/docs/site/",
			},
			plugins: [
				starlightOpenAPI([
					{
						base: "reference/api",
						sidebar: { label: "HTTP API" },
						schema: "../openapi/swagger.yaml",
					},
				]),
				starlightLlmsTxt(),
				starlightScrollToTop(),
			],
			sidebar: [
				{
					label: "Guide",
					items: [
						{ slug: "guide/getting-started" },
						{ slug: "guide/installation" },
						{ slug: "guide/devcontainers" },
					],
				},
				{
					label: "Features",
					autogenerate: { directory: "features" },
				},
				{
					label: "Integration",
					items: [
						{ slug: "integration/architecture" },
						{ slug: "integration/paths" },
						{ slug: "integration/http-api" },
						{ slug: "integration/go-client" },
						{ slug: "integration/go-library" },
					],
				},
				{
					label: "Reference",
					items: [
						...openAPISidebarGroups,
						{ slug: "reference/environment-variables" },
						{
							label: "Go Packages",
							collapsed: true,
							autogenerate: { directory: "reference/go" },
						},
					],
				},
				{ slug: "faq" },
				{ slug: "comparison" },
				{ slug: "contributing" },
				{ slug: "changelog" },
			],
			customCss: ["./src/styles/custom.css"],
			favicon: "/favicon.ico",
			lastUpdated: true,
		}),
	],
});
