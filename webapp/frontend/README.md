# Scrutiny Frontend

Angular 21 application for the Scrutiny Hard Drive Health Dashboard.

## Development server

```bash
npm install --legacy-peer-deps
npm run start -- --serve-path="/web/" --port 4200
```

Navigate to `http://localhost:4200/web/`. The app will automatically reload on source file changes.

## Build

```bash
npm run build:prod -- --output-path=../../dist
```

## Running unit tests

```bash
npm test -- --watch=false                             # Run tests once
npm test -- --watch=false --browsers=ChromeHeadless   # Headless CI mode
npx ng test --watch=false --code-coverage             # Run with coverage
```

## Linting

```bash
npm run lint
```
