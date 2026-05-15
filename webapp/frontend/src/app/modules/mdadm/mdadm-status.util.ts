export function getMdadmArrayStatusColorClass(stateValue?: string): string {
    const state = (stateValue || '').toLowerCase();
    if (state.includes('degraded') || state.includes('inactive')) {
        return 'text-red-600 dark:text-red-400 bg-red-100 dark:bg-red-900';
    }
    if (state.includes('checking') || state.includes('resync') || state.includes('recover') || state.includes('rebuild')) {
        return 'text-blue-600 dark:text-blue-400 bg-blue-100 dark:bg-blue-900';
    }
    if (state.includes('clean') || state.includes('active')) {
        return 'text-green-600 dark:text-green-400 bg-green-100 dark:bg-green-900';
    }
    return 'text-gray-600 dark:text-gray-400 bg-gray-100 dark:bg-gray-800';
}
