// Guards against Tailwind arbitrary-value classes such as 'min-w-[200px]'.
//
// src/styles/tailwind.scss is a pre-generated stylesheet and no Tailwind build
// runs over the templates, so an arbitrary-value class compiles to nothing.
// The template still builds and the element silently loses the style. That is
// how the notification channel fields collapsed into one unreadable row (#937).
//
// Use a utility that exists in src/styles/tailwind.scss, or a rule in the
// component's own stylesheet.

import { readFileSync, readdirSync } from 'node:fs';
import { dirname, extname, join, relative } from 'node:path';
import { fileURLToPath } from 'node:url';

const frontendRoot = dirname(dirname(fileURLToPath(import.meta.url)));
const srcRoot = join(frontendRoot, 'src');

function sourceFiles(directory) {
    return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
        const path = join(directory, entry.name);

        if (entry.isDirectory()) {
            return entry.name === 'styles' ? [] : sourceFiles(path);
        }

        return ['.html', '.ts'].includes(extname(entry.name)) ? [path] : [];
    });
}

// Matches class="..." and class='...', including values wrapped over several lines.
const classAttribute = /\bclass=(["'])([^"']*)\1/g;
const arbitraryValue = /^[!a-z0-9:&>_-]*-?\[[^\]]+\][a-z0-9:-]*$/;
const failures = [];

for (const file of sourceFiles(srcRoot)) {
    const source = readFileSync(file, 'utf8');

    for (const match of source.matchAll(classAttribute)) {
        const line = source.slice(0, match.index).split('\n').length;

        for (const token of match[2].split(/\s+/)) {
            if (arbitraryValue.test(token)) {
                failures.push(`${relative(frontendRoot, file)}:${line}: ${token}`);
            }
        }
    }
}

if (failures.length > 0) {
    console.error('Tailwind arbitrary-value classes have no CSS in the pre-generated stylesheet:');
    console.error(failures.join('\n'));
    console.error("\nUse a utility from src/styles/tailwind.scss or a rule in the component's stylesheet.");
    process.exitCode = 1;
} else {
    console.log('No Tailwind arbitrary-value classes in templates.');
}
