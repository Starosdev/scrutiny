import { TemperatureSelection } from './temperature-selection';

function createStorage(initialValue: string | null = null) {
    let value = initialValue;
    return {
        getItem: (_key: string) => value,
        setItem: (_key: string, nextValue: string) => {
            value = nextValue;
        },
        value: () => value,
    };
}

describe('TemperatureSelection', () => {
    it('starts empty, caps selection at twenty, and preserves existing selections', () => {
        const selection = new TemperatureSelection(createStorage());

        for (let index = 0; index < 21; index++) {
            selection.toggle(`device-${index}`);
        }

        expect(selection.ids.length).toBe(20);
        expect(selection.ids).toContain('device-0');
        expect(selection.ids).not.toContain('device-20');
    });

    it('removes an already selected device', () => {
        const selection = new TemperatureSelection(createStorage());
        selection.toggle('device-1');

        selection.toggle('device-1');

        expect(selection.ids).toEqual([]);
    });

    it('restores only available devices and caps stored selections at twenty', () => {
        const storedIDs = ['stale-device', ...Array.from({ length: 21 }, (_, index) => `device-${index}`)];
        const selection = new TemperatureSelection(createStorage(JSON.stringify(storedIDs)));

        selection.restore(Array.from({ length: 21 }, (_, index) => `device-${index}`));

        expect(selection.ids).toEqual(Array.from({ length: 20 }, (_, index) => `device-${index}`));
    });

    it('persists additions and removals', () => {
        const storage = createStorage();
        const selection = new TemperatureSelection(storage);

        selection.restore(['device-1']);
        selection.toggle('device-1');
        expect(storage.value()).toBe('["device-1"]');

        selection.toggle('device-1');
        expect(storage.value()).toBe('[]');
    });

    it('ignores malformed stored data', () => {
        const selection = new TemperatureSelection(createStorage('{malformed'));

        selection.restore(['device-1']);

        expect(selection.ids).toEqual([]);
    });
});
