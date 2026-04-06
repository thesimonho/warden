#!/usr/bin/env node

/**
 * Generates agent-friendly API reference markdown files from the OpenAPI spec.
 *
 * Uses @scalar/openapi-to-markdown for full schema expansion, then splits by
 * tag into per-resource files and strips repeated error response bodies.
 *
 * Usage:
 *   node docs/site/generate-api-docs.mjs
 *   node docs/site/generate-api-docs.mjs --input path/to/swagger.yaml --output path/to/api/
 */

import { readFileSync, writeFileSync, mkdirSync } from "fs";
import { resolve, dirname } from "path";
import { fileURLToPath } from "url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const projectRoot = resolve(__dirname, "../..");

// Parse CLI args
const args = process.argv.slice(2);
function getArg(name, fallback) {
	const idx = args.indexOf(name);
	if (idx === -1) return fallback;
	const value = args[idx + 1];
	if (!value || value.startsWith("--"))
		throw new Error(`Missing value for ${name}`);
	return value;
}

const inputPath = resolve(
	getArg("--input", resolve(projectRoot, "docs/openapi/swagger.yaml")),
);
const outputDir = resolve(
	getArg(
		"--output",
		resolve(projectRoot, "docs/plugin/skills/guide/reference/api"),
	),
);

// Tags that get folded into another tag's file
const TAG_MERGE = { clipboard: "projects" };

// Display names for tags
const TAG_TITLES = {
	access: "Access",
	audit: "Audit",
	containers: "Containers",
	health: "Health",
	host: "Host",
	projects: "Projects",
	settings: "Settings",
	streaming: "Streaming",
	worktrees: "Worktrees",
};

// Dynamic imports (ESM packages in node_modules)
const { createRequire } = await import("module");
const require = createRequire(import.meta.url);
const yaml = require("js-yaml");
const { createMarkdownFromOpenApi } =
	await import("@scalar/openapi-to-markdown");

// Generate full markdown from OpenAPI spec
const spec = yaml.load(readFileSync(inputPath, "utf8"));
const md = await createMarkdownFromOpenApi(spec);

// Split into per-endpoint sections by ### heading
const sections = md.split(/^(?=### )/m).filter((s) => s.startsWith("### "));

// Group by tag
const groups = {};
for (const section of sections) {
	// Scalar uses non-breaking space (\u00a0) after bold labels
	const tagMatch = section.match(/^- \*\*Tags:\*\*[\s\u00a0]+(.+)$/m);
	let tag = tagMatch ? tagMatch[1].trim() : "other";
	tag = TAG_MERGE[tag] || tag;
	if (!(tag in groups)) groups[tag] = [];
	groups[tag].push(section);
}

/**
 * Strip error response bodies from an endpoint section.
 * Keeps the status line (e.g. "##### Status: 400 Bad Request") but removes
 * everything underneath it until the next ##### or ## heading.
 */
function stripErrorBodies(text) {
	const lines = text.split("\n");
	const out = [];
	let skipping = false;

	for (const line of lines) {
		// Detect status headings
		const statusMatch = line.match(/^#{5}\s+Status:\s+(\d+)/);
		if (statusMatch) {
			const code = parseInt(statusMatch[1], 10);
			if (code >= 400) {
				// Keep the status line, start skipping body
				out.push(line);
				skipping = true;
				continue;
			}
			// Success status — stop skipping
			skipping = false;
		}

		// Stop skipping at next heading of same or higher level
		if (
			skipping &&
			(line.startsWith("## ") ||
				line.startsWith("### ") ||
				line.startsWith("##### "))
		) {
			// This line is a new heading — check if it's another error status
			const nextStatus = line.match(/^#{5}\s+Status:\s+(\d+)/);
			if (nextStatus && parseInt(nextStatus[1], 10) >= 400) {
				out.push(line);
				continue;
			}
			skipping = false;
		}

		if (skipping) continue;
		out.push(line);
	}

	return out.join("\n");
}

// Generate one file per tag
mkdirSync(outputDir, { recursive: true });

for (const [tag, endpoints] of Object.entries(groups)) {
	if (tag === "other") continue;
	if (!(tag in TAG_TITLES)) {
		console.warn(
			`WARNING: no title defined for tag "${tag}" — add it to TAG_TITLES`,
		);
	}
	const title = TAG_TITLES[tag] ?? tag.charAt(0).toUpperCase() + tag.slice(1);

	const header = [
		"<!-- GENERATED from docs/openapi/swagger.yaml — do not edit manually -->",
		"",
		`# ${title} API`,
		"",
		'All error responses return `{"error": "message", "code": "ERROR_CODE"}`.',
		"",
	].join("\n");

	// Promote ### to ## and strip error bodies
	const body = endpoints
		.map((s) => stripErrorBodies(s).replace(/^### /, "## "))
		.join("\n---\n\n");

	const outPath = resolve(outputDir, `${tag}.md`);
	writeFileSync(outPath, header + body + "\n");
	console.log(`  ${tag}.md (${endpoints.length} endpoints)`);
}

console.log(`Done. Generated ${Object.keys(groups).length} files.`);
