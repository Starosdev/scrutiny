import {Pipe, PipeTransform} from '@angular/core';

@Pipe({
    name: 'latency',
    standalone: false
})
export class LatencyPipe implements PipeTransform {

    static formatLatency(ns: number, dp: number = 1): string {
        if (ns == null || isNaN(ns)) {
            return '--';
        }
        if (ns < 1000) {
            return `${Math.round(ns)} ns`;
        }
        const us = ns / 1000;
        if (us < 1000) {
            return `${us.toFixed(dp)} us`;
        }
        const ms = us / 1000;
        if (ms < 1000) {
            return `${ms.toFixed(dp)} ms`;
        }
        const s = ms / 1000;
        return `${s.toFixed(dp)} s`;
    }

    transform(ns: number, dp: number = 1): string {
        return LatencyPipe.formatLatency(ns, dp);
    }
}
