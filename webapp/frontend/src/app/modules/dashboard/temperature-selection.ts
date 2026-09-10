const TEMPERATURE_SELECTION_STORAGE_KEY = 'scrutiny_temperature_selection';

type TemperatureSelectionStorage = Pick<Storage, 'getItem' | 'setItem'>;

export class TemperatureSelection {
    readonly maxSelected = 20;
    private readonly selected = new Set<string>();
    private hydrated = false;

    constructor(private readonly storage: TemperatureSelectionStorage | undefined = TemperatureSelection.browserStorage()) {}

    get ids(): string[] {
        return Array.from(this.selected);
    }

    has(deviceID: string): boolean {
        return this.selected.has(deviceID);
    }

    restore(availableDeviceIDs: readonly string[]): void {
        const available = new Set(availableDeviceIDs);
        if (!this.hydrated) {
            this.hydrated = true;
            for (const deviceID of this.readStoredIDs()) {
                if (available.has(deviceID)) {
                    this.selected.add(deviceID);
                }
                if (this.selected.size >= this.maxSelected) {
                    break;
                }
            }
        }

        let changed = false;
        for (const deviceID of this.selected) {
            if (!available.has(deviceID)) {
                this.selected.delete(deviceID);
                changed = true;
            }
        }
        if (changed) {
            this.persist();
        }
    }

    toggle(deviceID: string): void {
        if (this.selected.delete(deviceID)) {
            this.persist();
            return;
        }
        if (this.selected.size < this.maxSelected) {
            this.selected.add(deviceID);
            this.persist();
        }
    }

    private readStoredIDs(): string[] {
        try {
            const stored = this.storage?.getItem(TEMPERATURE_SELECTION_STORAGE_KEY);
            if (!stored) {
                return [];
            }
            const parsed: unknown = JSON.parse(stored);
            if (!Array.isArray(parsed)) {
                return [];
            }
            return parsed.filter((deviceID): deviceID is string => typeof deviceID === 'string' && deviceID.length > 0);
        } catch {
            return [];
        }
    }

    private persist(): void {
        try {
            this.storage?.setItem(TEMPERATURE_SELECTION_STORAGE_KEY, JSON.stringify(this.ids));
        } catch {
            // Selection persistence is optional; keep in-memory behavior when storage is unavailable.
        }
    }

    private static browserStorage(): TemperatureSelectionStorage | undefined {
        try {
            return globalThis.localStorage;
        } catch {
            return undefined;
        }
    }
}
