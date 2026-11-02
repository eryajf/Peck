#!/usr/bin/env node

import fs from "node:fs";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const dataPath = path.join(root, "data", "project-links.json");
const readmePath = path.join(root, "README.md");
const beginMarker = "<!-- BEGIN PROJECT LINKS -->";
const endMarker = "<!-- END PROJECT LINKS -->";
const bundleIDPattern = /^[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*$/;

function fail(message) {
  throw new Error(message);
}

function requireString(value, field, index, { maxLength = Infinity } = {}) {
  if (typeof value !== "string" || value.trim() === "") {
    fail(`projects[${index}].${field} must be a non-empty string`);
  }
  if (value.length > maxLength) {
    fail(`projects[${index}].${field} must be at most ${maxLength} characters`);
  }
  return value;
}

function requireHTTPS(value, field, index) {
  requireString(value, field, index);
  let url;
  try {
    url = new URL(value);
  } catch {
    fail(`projects[${index}].${field} must be a valid HTTPS URL`);
  }
  if (url.protocol !== "https:" || url.username || url.password) {
    fail(`projects[${index}].${field} must be an HTTPS URL without credentials`);
  }
  return url.href;
}

function loadProjects() {
  let data;
  try {
    data = JSON.parse(fs.readFileSync(dataPath, "utf8"));
  } catch (error) {
    fail(`unable to read ${path.relative(root, dataPath)}: ${error.message}`);
  }
  if (!data || !Array.isArray(data.projects)) {
    fail("data/project-links.json must contain a projects array");
  }

  const names = new Set();
  const bundleIDs = new Set();
  for (const [index, project] of data.projects.entries()) {
    if (!project || typeof project !== "object" || Array.isArray(project)) {
      fail(`projects[${index}] must be an object`);
    }
    const name = requireString(project.name, "name", index, { maxLength: 128 });
    const description = requireString(project.description, "description", index, { maxLength: 240 });
    project.description_en = requireString(project.description_en, "description_en", index, { maxLength: 240 });
    project.homepage = requireHTTPS(project.homepage, "homepage", index);
    project.icon = requireHTTPS(project.icon, "icon", index);
    project.status = requireString(project.status, "status", index, { maxLength: 512 });
    project.status_url = requireHTTPS(project.status_url, "status_url", index);
    const bundleID = requireString(project.bundle_id, "bundle_id", index, { maxLength: 255 });
    if (!bundleIDPattern.test(bundleID)) {
      fail(`projects[${index}].bundle_id is not a valid macOS bundle identifier`);
    }
    if (project.release !== undefined) {
      requireString(project.release, "release", index, { maxLength: 64 });
    }
    if (names.has(name)) fail(`duplicate project name: ${name}`);
    if (bundleIDs.has(bundleID)) fail(`duplicate bundle_id: ${bundleID}`);
    names.add(name);
    bundleIDs.add(bundleID);
    project.description = description;
  }
  return data.projects;
}

function escapeTable(value) {
  return escapeHTML(value).replaceAll("|", "\\|").replaceAll("\r", " ").replaceAll("\n", " ");
}

function escapeHTML(value) {
  return value.replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;").replaceAll('"', "&quot;").replaceAll("'", "&#39;");
}

function deeplink(project) {
  const params = new URLSearchParams({ v: "1", project: project.name, bundle_id: project.bundle_id });
  if (project.release) params.set("release", project.release);
  params.set("homepage", project.homepage);
  return `peck://prepare?${params.toString()}`;
}

function render(projects) {
  const lines = [
    beginMarker,
    "",
    "| 项目 | 说明 | 签名与隔离标记核对 | Deeplink 跳转 |",
    "| --- | --- | --- | --- |",
  ];
  for (const project of projects) {
    const openLabel = escapeHTML(`使用 Peck 打开 ${project.name}`);
    const status = `${escapeTable(project.status)}（[依据](${escapeHTML(project.status_url)})）`;
    const link = deeplink(project);
    lines.push(
      `| [${escapeTable(project.name)}](${project.homepage}) | ${escapeTable(project.description)} | ${status} | [<img src="assets/peck-icon.svg" width="24" alt="${openLabel}"> 使用 Peck 处理](${link} "${openLabel}")<br><code>${escapeHTML(link)}</code> |`,
    );
  }
  lines.push("", endMarker);
  return lines.join("\n");
}

function updateReadme(content, generated) {
  const markerPattern = new RegExp(`${beginMarker}[\\s\\S]*?${endMarker}`);
  if (!markerPattern.test(content)) {
    fail(`README.md is missing the managed markers ${beginMarker} and ${endMarker}`);
  }
  return content.replace(markerPattern, generated);
}

function main() {
  const mode = process.argv.slice(2).find((arg) => ["--validate", "--check", "--write"].includes(arg)) || "--check";
  const projects = loadProjects();
  const generated = render(projects);
  const current = fs.readFileSync(readmePath, "utf8");
  const updated = updateReadme(current, generated);

  if (mode === "--validate") {
    console.log(`Validated ${projects.length} project entries.`);
    return;
  }
  if (mode === "--check") {
    if (updated !== current) {
      fail("README.md is out of date; run node .github/scripts/render-project-links.mjs --write");
    }
    console.log(`README.md is up to date with ${projects.length} project entries.`);
    return;
  }
  if (updated !== current) {
    fs.writeFileSync(readmePath, updated);
    console.log(`Updated README.md with ${projects.length} project entries.`);
  } else {
    console.log(`README.md already contains ${projects.length} project entries.`);
  }
}

try {
  main();
} catch (error) {
  console.error(`Error: ${error.message}`);
  process.exitCode = 1;
}
