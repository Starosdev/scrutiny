// Flat config, required since angular-eslint 22: the builder no longer reads
// .eslintrc.json and ESLINT_USE_FLAT_CONFIG=false does not reach it. Rules are
// carried over unchanged from the eslintrc this replaces.
const eslint = require('@eslint/js');
const tseslint = require('typescript-eslint');
const angular = require('angular-eslint');
const prettier = require('eslint-plugin-prettier/recommended');

module.exports = tseslint.config(
    {
        ignores: ['projects/**/*', 'dist/**/*', 'node_modules/**/*', 'src/tailwind/**/*', 'out-tsc/**/*'],
    },
    {
        files: ['**/*.ts'],
        extends: [
            eslint.configs.recommended,
            ...tseslint.configs.recommended,
            ...angular.configs.tsRecommended,
            prettier,
        ],
        processor: angular.processInlineTemplates,
        rules: {
            '@angular-eslint/directive-selector': [
                'error',
                {
                    type: 'attribute',
                    prefix: ['treo', 'app'],
                    style: 'camelCase',
                },
            ],
            '@angular-eslint/component-selector': 'off',
            // New in the angular-eslint 22 preset, and it fires on every
            // component the Angular 22 migration marked Eager to keep its
            // existing change detection. Moving those to OnPush is a behavior
            // change, so it is left for separate work.
            '@angular-eslint/prefer-on-push-component-change-detection': 'off',
            '@angular-eslint/no-input-rename': 'off',
            '@angular-eslint/no-output-native': 'off',
            '@typescript-eslint/no-explicit-any': 'off',
            '@typescript-eslint/no-unused-vars': ['warn', { argsIgnorePattern: '^_' }],
            '@typescript-eslint/no-non-null-assertion': 'off',
            'prettier/prettier': 'warn',
        },
    },
    {
        files: ['**/*.html'],
        extends: [...angular.configs.templateRecommended],
        rules: {},
    },
);
