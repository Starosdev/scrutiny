export class TreoMockApiUtils {
    /**
     * Constructor
     */
    constructor() {}

    // -----------------------------------------------------------------------------------------------------
    // @ Public methods
    // -----------------------------------------------------------------------------------------------------

    /**
     * Generate a globally unique id
     */
    static guid(): string {
        return crypto.randomUUID();
    }
}
