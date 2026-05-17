import { Component, OnInit, ViewEncapsulation, inject } from '@angular/core';
import { FormBuilder, FormGroup, Validators, ReactiveFormsModule } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';
import { AuthService } from 'app/core/auth/auth.service';
import { take } from 'rxjs/operators';
import { MatProgressBar } from '@angular/material/progress-bar';
import { MatIcon } from '@angular/material/icon';
import { MatTabGroup, MatTab } from '@angular/material/tabs';
import { MatFormField, MatLabel, MatSuffix } from '@angular/material/form-field';
import { MatInput } from '@angular/material/input';
import { MatIconButton, MatButton } from '@angular/material/button';

@Component({
    selector: 'login',
    templateUrl: './login.component.html',
    styleUrls: ['./login.component.scss'],
    encapsulation: ViewEncapsulation.None,
    imports: [MatProgressBar, MatIcon, MatTabGroup, MatTab, ReactiveFormsModule, MatFormField, MatLabel, MatInput, MatIconButton, MatSuffix, MatButton],
})
export class LoginComponent implements OnInit {
    private readonly _fb = inject(FormBuilder);
    private readonly _authService = inject(AuthService);
    private readonly _router = inject(Router);
    private readonly _route = inject(ActivatedRoute);

    tokenForm: FormGroup;
    passwordForm: FormGroup;
    errorMessage: string = '';
    isLoading: boolean = false;
    loginMethods: string[] = [];
    showPasswordTab: boolean = false;
    hideToken: boolean = true;
    hidePassword: boolean = true;

    private returnUrl: string = '/dashboard';

    constructor() {
        this.tokenForm = this._fb.group({
            token: ['', Validators.required],
        });

        this.passwordForm = this._fb.group({
            username: ['', Validators.required],
            password: ['', Validators.required],
        });
    }

    ngOnInit(): void {
        // If auth is disabled or already logged in, redirect away
        if (!this._authService.authEnabled || this._authService.isLoggedIn) {
            this._router.navigate(['/dashboard']);
            return;
        }

        this.returnUrl = this.sanitizeReturnUrl(this._route.snapshot.queryParams['returnUrl']);

        this._authService.loginMethods$.pipe(take(1)).subscribe((methods) => {
            this.loginMethods = methods;
            this.showPasswordTab = methods.includes('password');
        });
    }

    onTokenLogin(): void {
        if (this.tokenForm.invalid) {
            return;
        }

        this.isLoading = true;
        this.errorMessage = '';

        this._authService.loginWithToken(this.tokenForm.value.token).subscribe({
            next: (res) => {
                this.isLoading = false;
                if (res.success && res.token) {
                    this._router.navigateByUrl(this.returnUrl);
                } else if (!res.success) {
                    this.errorMessage = res.error || 'Login failed. Please try again.';
                }
            },
            error: (err) => {
                this.isLoading = false;
                this.errorMessage = err.error?.error || 'Login failed. Please try again.';
            },
        });
    }

    onPasswordLogin(): void {
        if (this.passwordForm.invalid) {
            return;
        }

        this.isLoading = true;
        this.errorMessage = '';

        const { username, password } = this.passwordForm.value;
        this._authService.loginWithPassword(username, password).subscribe({
            next: (res) => {
                this.isLoading = false;
                if (res.success && res.token) {
                    this._router.navigateByUrl(this.returnUrl);
                } else if (!res.success) {
                    this.errorMessage = res.error || 'Login failed. Please try again.';
                }
            },
            error: (err) => {
                this.isLoading = false;
                this.errorMessage = err.error?.error || 'Login failed. Please try again.';
            },
        });
    }

    private sanitizeReturnUrl(url: string): string {
        if (!url || !url.startsWith('/') || url.startsWith('//')) {
            return '/dashboard';
        }
        return url;
    }
}
