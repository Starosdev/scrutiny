import { Component, Input, ChangeDetectionStrategy } from '@angular/core';
import { MatIconButton } from '@angular/material/button';
import { MatIcon } from '@angular/material/icon';
import { MatTooltip } from '@angular/material/tooltip';

@Component({
    selector: 'app-help-link-icon',
    templateUrl: './help-link-icon.component.html',
    changeDetection: ChangeDetectionStrategy.Eager,
    imports: [MatIconButton, MatIcon, MatTooltip],
})
export class HelpLinkIconComponent {
    @Input() href: string = '';
    @Input() tooltip: string = 'View documentation';
}
