// Guards the Treo -> Angular Material theme adapter.
//
// Angular Material derives every component color token from '_mat-system', a
// flat map that 'mat.m2-define-*-theme' attaches to the theme. A theme adapter
// that rebuilds the theme map instead of merging into it drops that key, every
// token resolves to null, and the generated stylesheet silently emits no color
// custom properties at all. Overlay-rendered components then render on a
// transparent background (#541), and nothing in the build reports an error.
//
// This check compiles the real theme entrypoint and asserts that a
// representative token still lands on each generated theme class.

import { existsSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const frontendRoot = dirname(dirname(fileURLToPath(import.meta.url)));
const entrypoint = join(frontendRoot, 'src', '@treo', 'styles', 'main.scss');
const nodeModules = join(frontendRoot, 'node_modules');

// Every theme class generated from '$treo-themes' in 'src/styles/themes.scss'.
const themeClasses = ['treo-theme-dark', 'treo-theme-light', 'theme-dark', 'theme-light'];

// One token per component named in the acceptance criteria of #543, plus the
// app surface. Each resolves from a different system key, so a partial
// regression cannot pass.
const requiredTokens = [
    '--mat-select-panel-background-color',
    '--mat-menu-container-color',
    '--mat-autocomplete-background-color',
    '--mat-dialog-container-color',
    '--mat-expansion-container-background-color',
    '--mat-option-label-text-color',
    '--mat-app-background-color',
    '--mat-form-field-filled-container-color',
];

if (!existsSync(join(nodeModules, 'sass'))) {
    console.error("Cannot find 'sass' in node_modules. Run 'npm ci' in webapp/frontend first.");
    process.exit(1);
}

const sass = await import('sass');

const css = sass.compile(entrypoint, {
    loadPaths: [nodeModules, join(frontendRoot, 'src', '@treo', 'styles')],
    silenceDeprecations: ['if-function', 'import', 'global-builtin', 'color-functions'],
}).css;

// Walk top-level rules. Nested selectors such as '.treo-theme-dark .mat-warn'
// set the same tokens with palette variants, so matching on the theme class
// alone would read a descendant's value.
function* topLevelRules(source) {
    let index = 0;

    while (index < source.length) {
        const open = source.indexOf('{', index);

        if (open < 0) {
            return;
        }

        const selector = source.slice(index, open).trim();
        let depth = 1;
        let cursor = open + 1;

        while (cursor < source.length && depth > 0) {
            if (source[cursor] === '{') {
                depth += 1;
            } else if (source[cursor] === '}') {
                depth -= 1;
            }

            cursor += 1;
        }

        yield [selector, source.slice(open + 1, cursor - 1)];
        index = cursor;
    }
}

const tokensByTheme = new Map(themeClasses.map((name) => [name, new Map()]));

for (const [selector, body] of topLevelRules(css)) {
    for (const part of selector.split(',')) {
        const themeClass = part.trim().slice(1);

        if (!tokensByTheme.has(themeClass) || part.trim() !== `.${themeClass}`) {
            continue;
        }

        for (const match of body.matchAll(/(--mat-[a-z0-9-]+):\s*([^;]+);/g)) {
            tokensByTheme.get(themeClass).set(match[1], match[2].trim());
        }
    }
}

const failures = [];

for (const themeClass of themeClasses) {
    const tokens = tokensByTheme.get(themeClass);

    if (tokens.size === 0) {
        failures.push(`.${themeClass} emits no --mat-* tokens at all`);
        continue;
    }

    for (const token of requiredTokens) {
        if (!tokens.has(token)) {
            failures.push(`.${themeClass} is missing ${token}`);
        }
    }
}

if (failures.length > 0) {
    console.error('The generated Angular Material theme is missing color tokens:');
    console.error(failures.join('\n'));
    console.error(
        "\nThe theme adapter in src/@treo/styles/utilities/_colors.scss must merge into the" +
        "\nAngular Material theme map, never rebuild it, so that '_mat-system' survives." +
        '\nSee _treo-modify-angular-material-theme-colors and _treo-system-color-overrides.'
    );
    process.exitCode = 1;
} else {
    const counts = themeClasses.map((name) => `${name}=${tokensByTheme.get(name).size}`).join(' ');
    console.log(`Angular Material theme tokens present on every theme class (${counts}).`);
}
