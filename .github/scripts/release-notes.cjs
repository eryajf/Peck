function stripManagedSections(body) {
  const lines = (body || "").split(/\r?\n/);
  const kept = [];
  let skippingManagedSection = false;

  for (const line of lines) {
    const trimmed = line.trim();

    if (/^## (Downloads|Checksums)$/.test(trimmed)) {
      skippingManagedSection = true;
      continue;
    }

    if (skippingManagedSection && /^##\s+/.test(trimmed)) {
      skippingManagedSection = false;
    }

    if (!skippingManagedSection) {
      kept.push(line);
    }
  }

  return kept.join("\n").trim();
}

// release-drafter may place the same PR in multiple categories when several
// labels match. Keep the first category entry only.
function dedupeChangeEntries(body) {
  const lines = (body || "").split(/\r?\n/);
  const output = [];
  let inChanges = false;
  let currentCategoryHeading = null;
  let currentCategoryLines = [];
  const seenEntries = new Set();

  const isListItem = (line) => /^\s*-\s+/.test(line);
  const entryKeyFor = (line) => {
    const prNumber = line.match(/\(#(\d+)\)/);
    if (prNumber) {
      return `pr:${prNumber[1]}`;
    }

    return `line:${line.trim().toLowerCase()}`;
  };
  const trimBlankEdges = (sectionLines) => {
    const trimmed = [...sectionLines];
    while (trimmed.length > 0 && trimmed[0].trim() === "") {
      trimmed.shift();
    }
    while (trimmed.length > 0 && trimmed[trimmed.length - 1].trim() === "") {
      trimmed.pop();
    }
    return trimmed;
  };
  const flushCategory = () => {
    if (!currentCategoryHeading) {
      return;
    }

    const keptLines = trimBlankEdges(currentCategoryLines);
    const hasContent = keptLines.some((line) => line.trim() !== "");

    if (hasContent) {
      if (output.length > 0 && output[output.length - 1].trim() !== "") {
        output.push("");
      }
      output.push(currentCategoryHeading);
      output.push("");
      output.push(...keptLines);
    }

    currentCategoryHeading = null;
    currentCategoryLines = [];
  };

  for (const line of lines) {
    const trimmed = line.trim();

    if (!inChanges) {
      output.push(line);
      if (trimmed === "## Changes") {
        inChanges = true;
      }
      continue;
    }

    if (/^##\s+/.test(trimmed)) {
      flushCategory();
      if (output.length > 0 && output[output.length - 1].trim() !== "") {
        output.push("");
      }
      output.push(line);
      inChanges = false;
      continue;
    }

    if (/^###\s+/.test(trimmed)) {
      flushCategory();
      currentCategoryHeading = trimmed;
      currentCategoryLines = [];
      continue;
    }

    if (!currentCategoryHeading) {
      output.push(line);
      continue;
    }

    if (isListItem(line)) {
      const entryKey = entryKeyFor(line);
      if (seenEntries.has(entryKey)) {
        continue;
      }
      seenEntries.add(entryKey);
    }

    currentCategoryLines.push(line);
  }

  flushCategory();

  return output
    .join("\n")
    .replace(/\n{3,}/g, "\n\n")
    .trim();
}

module.exports = {
  dedupeChangeEntries,
  stripManagedSections,
};
