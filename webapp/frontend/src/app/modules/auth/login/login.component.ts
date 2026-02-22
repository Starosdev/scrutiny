import { Component, OnInit, ViewEncapsulation } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';
import { AuthService } from 'app/core/auth/auth.service';

@Component({
    selector: 'login',
    templateUrl: './login.component.html',
    styleUrls: ['./login.component.scss'],
    encapsulation: ViewEncapsulation.None,
    standalone: false
})
export class LoginComponent implements OnInit {
    tokenForm: FormGroup;
    passwordForm: FormGroup;
    errorMessage: string = '';
    isLoading: boolean = false;
    loginMethods: string[] = [];
    showPasswordTab: boolean = false;
    hideToken: boolean = true;
    hidePassword: boolean = true;

    private returnUrl: string = '/dashboard';

    constructor(
        private _fb: FormBuilder,
        private _authService: AuthService,
        private _router: Router,
        private _route: ActivatedRoute
    ) {
        this.tokenForm = this._fb.group({
            token: ['', Validators.required]
        });

        this.passwordForm = this._fb.group({
            username: ['', Validators.required],
            password: ['', Validators.required]
        });
    }

    ngOnInit(): void {
        // If auth is disabled or already logged in, redirect away
        if (!this._authService.authEnabled || this._authService.isLoggedIn) {
            this._router.navigate(['/dashboard']);
            return;
        }

        this.returnUrl = this._route.snapshot.queryParams['returnUrl'] || '/dashboard';

        this._authService.loginMethods$.subscribe(methods => {
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
                }
            },
            error: (err) => {
                this.isLoading = false;
                this.errorMessage = err.error?.error || 'Login failed. Please try again.';
            }
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
                }
            },
            error: (err) => {
                this.isLoading = false;
                this.errorMessage = err.error?.error || 'Login failed. Please try again.';
            }
        });
    }
}
