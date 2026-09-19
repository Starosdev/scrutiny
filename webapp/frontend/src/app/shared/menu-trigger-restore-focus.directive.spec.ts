import { Component, ElementRef, ViewChild, ChangeDetectionStrategy } from '@angular/core';
import { ComponentFixture, TestBed, fakeAsync, flush } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { NoopAnimationsModule } from '@angular/platform-browser/animations';
import { OverlayContainer } from '@angular/cdk/overlay';
import { MatButtonModule } from '@angular/material/button';
import { MatMenuModule, MatMenuTrigger } from '@angular/material/menu';
import { MenuTriggerRestoreFocusDirective } from './menu-trigger-restore-focus.directive';

// The trigger sits between two tall spacers inside its own scroll container, so the assertions do not
// depend on the size of the karma browser window. Focusing an element scrolls every scrollable
// ancestor into view, and { preventScroll: true } suppresses all of them.
const SCROLLER_TEMPLATE = `
    <div #scroller style="height: 120px; overflow-y: auto;">
        <div style="height: 600px;"></div>
        <button mat-icon-button [matMenuTriggerFor]="menu">Open</button>
        <div style="height: 600px;"></div>
    </div>
    <mat-menu #menu="matMenu">
        <button mat-menu-item>Item</button>
    </mat-menu>
`;

@Component({
    selector: 'app-patched-menu-host',
    template: SCROLLER_TEMPLATE,
    changeDetection: ChangeDetectionStrategy.Eager,
    imports: [MatButtonModule, MatMenuModule, MenuTriggerRestoreFocusDirective],
})
class PatchedMenuHostComponent {
    @ViewChild('scroller', { static: true }) scroller!: ElementRef<HTMLElement>;
    @ViewChild(MatMenuTrigger, { static: true }) trigger!: MatMenuTrigger;
}

// Control host: identical, but without the directive. Proves the specs above can actually detect the
// bug, and fails loudly if Angular Material ever passes { preventScroll: true } itself - at which
// point the directive can be deleted.
@Component({
    selector: 'app-unpatched-menu-host',
    template: SCROLLER_TEMPLATE,
    changeDetection: ChangeDetectionStrategy.Eager,
    imports: [MatButtonModule, MatMenuModule],
})
class UnpatchedMenuHostComponent {
    @ViewChild('scroller', { static: true }) scroller!: ElementRef<HTMLElement>;
    @ViewChild(MatMenuTrigger, { static: true }) trigger!: MatMenuTrigger;
}

@Component({
    selector: 'app-opted-out-menu-host',
    template: `
        <button mat-icon-button [matMenuTriggerFor]="menu" [matMenuTriggerRestoreFocus]="false">Open</button>
        <mat-menu #menu="matMenu">
            <button mat-menu-item>Item</button>
        </mat-menu>
    `,
    changeDetection: ChangeDetectionStrategy.Eager,
    imports: [MatButtonModule, MatMenuModule, MenuTriggerRestoreFocusDirective],
})
class OptedOutMenuHostComponent {
    @ViewChild(MatMenuTrigger, { static: true }) trigger!: MatMenuTrigger;
}

@Component({
    selector: 'app-submenu-host',
    template: `
        <button mat-icon-button [matMenuTriggerFor]="rootMenu">Open</button>
        <mat-menu #rootMenu="matMenu">
            <button mat-menu-item [matMenuTriggerFor]="subMenu">Sub</button>
        </mat-menu>
        <mat-menu #subMenu="matMenu">
            <button mat-menu-item>Leaf</button>
        </mat-menu>
    `,
    changeDetection: ChangeDetectionStrategy.Eager,
    imports: [MatButtonModule, MatMenuModule, MenuTriggerRestoreFocusDirective],
})
class SubmenuHostComponent {}

describe('MenuTriggerRestoreFocusDirective', () => {
    afterEach(() => {
        TestBed.inject(OverlayContainer).ngOnDestroy();
    });

    describe('with the directive applied', () => {
        let fixture: ComponentFixture<PatchedMenuHostComponent>;
        let host: PatchedMenuHostComponent;
        let triggerElement: HTMLElement;
        let focusSpy: jasmine.Spy;

        beforeEach(() => {
            TestBed.configureTestingModule({
                imports: [NoopAnimationsModule, MatButtonModule, MatMenuModule, PatchedMenuHostComponent],
            }).compileComponents();

            fixture = TestBed.createComponent(PatchedMenuHostComponent);
            host = fixture.componentInstance;
            fixture.detectChanges();

            triggerElement = fixture.debugElement.query(By.directive(MatMenuTrigger)).nativeElement;
            focusSpy = spyOn(triggerElement, 'focus').and.callThrough();
        });

        it('restores focus to the trigger without scrolling it back into view', fakeAsync(() => {
            host.scroller.nativeElement.scrollTop = 600;

            host.trigger.openMenu();
            fixture.detectChanges();
            flush();

            host.scroller.nativeElement.scrollTop = 0;

            host.trigger.closeMenu();
            fixture.detectChanges();
            flush();

            expect(focusSpy).toHaveBeenCalledOnceWith({ preventScroll: true });
            expect(host.scroller.nativeElement.scrollTop).toBe(0);
            expect(document.activeElement).toBe(triggerElement);
        }));

        it('takes focus restoration over from the menu trigger', fakeAsync(() => {
            expect(host.trigger.restoreFocus).toBeTrue();

            host.trigger.openMenu();
            fixture.detectChanges();
            flush();

            expect(host.trigger.restoreFocus).toBeFalse();
        }));
    });

    describe('without the directive applied', () => {
        let fixture: ComponentFixture<UnpatchedMenuHostComponent>;
        let host: UnpatchedMenuHostComponent;
        let focusSpy: jasmine.Spy;

        beforeEach(() => {
            TestBed.configureTestingModule({
                imports: [NoopAnimationsModule, MatButtonModule, MatMenuModule, UnpatchedMenuHostComponent],
            }).compileComponents();

            fixture = TestBed.createComponent(UnpatchedMenuHostComponent);
            host = fixture.componentInstance;
            fixture.detectChanges();

            focusSpy = spyOn(fixture.debugElement.query(By.directive(MatMenuTrigger)).nativeElement, 'focus').and.callThrough();
        });

        it('reproduces the scroll jump, so the specs above are not tautological', fakeAsync(() => {
            host.scroller.nativeElement.scrollTop = 600;

            host.trigger.openMenu();
            fixture.detectChanges();
            flush();

            host.scroller.nativeElement.scrollTop = 0;

            host.trigger.closeMenu();
            fixture.detectChanges();
            flush();

            expect(focusSpy).toHaveBeenCalledOnceWith(undefined);
            expect(host.scroller.nativeElement.scrollTop).toBeGreaterThan(0);
        }));
    });

    describe('when the template opts out of focus restoration', () => {
        it('does not restore focus', fakeAsync(() => {
            TestBed.configureTestingModule({
                imports: [NoopAnimationsModule, MatButtonModule, MatMenuModule, OptedOutMenuHostComponent],
            }).compileComponents();

            const fixture = TestBed.createComponent(OptedOutMenuHostComponent);
            fixture.detectChanges();

            const focusSpy = spyOn(fixture.debugElement.query(By.directive(MatMenuTrigger)).nativeElement, 'focus').and.callThrough();

            fixture.componentInstance.trigger.openMenu();
            fixture.detectChanges();
            flush();

            fixture.componentInstance.trigger.closeMenu();
            fixture.detectChanges();
            flush();

            expect(focusSpy).not.toHaveBeenCalled();
        }));
    });

    describe('with a submenu trigger', () => {
        it('leaves the submenu trigger to Angular Material', fakeAsync(() => {
            TestBed.configureTestingModule({
                imports: [NoopAnimationsModule, MatButtonModule, MatMenuModule, SubmenuHostComponent],
            }).compileComponents();

            const fixture = TestBed.createComponent(SubmenuHostComponent);
            fixture.detectChanges();

            const rootTrigger = fixture.debugElement.query(By.directive(MatMenuTrigger)).injector.get(MatMenuTrigger);

            rootTrigger.openMenu();
            fixture.detectChanges();
            flush();

            // The submenu trigger only exists once the root menu has rendered its content
            const subTrigger = fixture.debugElement
                .queryAll(By.directive(MatMenuTrigger))
                .map((debugElement) => debugElement.injector.get(MatMenuTrigger))
                .find((trigger) => trigger.triggersSubmenu());

            expect(subTrigger).toBeDefined();

            subTrigger!.openMenu();
            fixture.detectChanges();
            flush();

            expect(subTrigger!.restoreFocus).toBeTrue();

            rootTrigger.closeMenu();
            fixture.detectChanges();
            flush();
        }));
    });

    describe('when the trigger is detached before the menu closes', () => {
        it('does not try to restore focus', fakeAsync(() => {
            TestBed.configureTestingModule({
                imports: [NoopAnimationsModule, MatButtonModule, MatMenuModule, PatchedMenuHostComponent],
            }).compileComponents();

            const fixture = TestBed.createComponent(PatchedMenuHostComponent);
            const host = fixture.componentInstance;
            fixture.detectChanges();

            const triggerElement: HTMLElement = fixture.debugElement.query(By.directive(MatMenuTrigger)).nativeElement;
            const focusSpy = spyOn(triggerElement, 'focus').and.callThrough();

            host.trigger.openMenu();
            fixture.detectChanges();
            flush();

            // Stands in for a menu item that navigated away and took the trigger with it
            triggerElement.remove();

            expect(() => {
                host.trigger.closeMenu();
                fixture.detectChanges();
                flush();
            }).not.toThrow();

            expect(focusSpy).not.toHaveBeenCalled();
        }));
    });
});
