package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"clientroll/internal/auth"
	"clientroll/internal/config"
	"clientroll/internal/database"
	"clientroll/internal/email"
	"clientroll/internal/events"
	"clientroll/internal/handlers"
	"clientroll/internal/middleware"
	"clientroll/internal/observability"
	"clientroll/internal/repository"
	"clientroll/internal/storage"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := database.NewPostgres(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer db.Close()

	rdb, err := database.NewRedis(cfg.RedisURL)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer rdb.Close()

	enc, err := database.NewEncryptor(cfg.MasterEncryptionKey)
	if err != nil {
		log.Fatalf("Failed to initialise encryptor: %v", err)
	}

	r := setupRouter(cfg, db, rdb, enc)
	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()
	go events.NewWorker(db, rdb, enc).Start(workerCtx)
	go observability.StartAlertsMonitor(workerCtx, db, 2*time.Minute)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("ClientRoll API starting on port %d", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	workerCancel()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server stopped")
}

func setupRouter(cfg *config.Config, db *database.Postgres, rdb *database.Redis, enc *database.Encryptor) *chi.Mux {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(middleware.NewLogger().Handler)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Compress(5))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSAllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Initialise auth components
	jwtMgr := auth.NewJWTManager(cfg.MasterEncryptionKey, cfg.JWTAccessDuration, cfg.JWTRefreshDuration)
	sessionMgr := auth.NewSessionManager(rdb, cfg.JWTRefreshDuration)
	authMW := middleware.NewAuthMiddleware(jwtMgr, sessionMgr)
	tenantMW := middleware.NewTenantMiddleware(db)
	rateLimiter := middleware.NewAuthRateLimiter(rdb)
	loginLimiter := middleware.NewLoginRateLimiter(rdb)
	auditMW := middleware.NewAuditMiddleware()

	// Repositories
	userRepo := repository.NewUserRepo(db)
	adminRepo := repository.NewPlatformAdminRepo(db)
	tenantRepo := repository.NewTenantRepo(db)
	settingsRepo := repository.NewSettingsRepo(db)
	bonusRepo := repository.NewBonusRepo(db)
	mtRepo := repository.NewMembershipTypeRepo(db)
	termsRepo := repository.NewTermsRepo(db)
	clientRepo := repository.NewClientRepo(db)
	memberRepo := repository.NewMembershipRepo(db)
	paymentRepo := repository.NewPaymentRepo(db)
	paymentDisputeRepo := repository.NewPaymentDisputeRepo(db)
	reportsRepo := repository.NewReportsRepo(db)
	webhookRepo := repository.NewWebhookRepo(db)
	inboundWebhookEventRepo := repository.NewInboundWebhookEventRepo(db)
	inboundWebhookKeyRepo := repository.NewInboundWebhookKeyRepo(db)
	credRepo := repository.NewTenantCredentialRepo(db)
	platformBillingConfigRepo := repository.NewPlatformBillingConfigRepo(db)
	tenantSubRepo := repository.NewTenantSubscriptionRepo(db)
	tenantUsageRepo := repository.NewTenantUsageRepo(db)
	tenantInvoiceRepo := repository.NewTenantInvoiceRepo(db)
	tenantEmailTemplateRepo := repository.NewTenantEmailTemplateRepo(db)

	// Email service
	emailSvc := email.NewMailerSendProvider(cfg.MailerSendAPIKey, cfg.MailerSendFromEmail, "ClientRoll")

	// Handlers
	authHandler := handlers.NewAuthHandler(userRepo, adminRepo, tenantRepo, jwtMgr, sessionMgr)
	platformHandler := handlers.NewPlatformHandler(tenantRepo, userRepo)
	platformBillingHandler := handlers.NewPlatformBillingHandler(cfg.AppURL, enc, platformBillingConfigRepo, tenantRepo, tenantSubRepo, tenantUsageRepo, tenantInvoiceRepo)
	platformBillingWebhookHandler := handlers.NewPlatformBillingWebhookHandler(enc, platformBillingConfigRepo, tenantSubRepo, tenantInvoiceRepo)
	settingsHandler := handlers.NewSettingsHandler(settingsRepo, tenantRepo, tenantEmailTemplateRepo)
	staffHandler := handlers.NewStaffHandler(userRepo)
	bonusHandler := handlers.NewBonusHandler(bonusRepo)
	mtHandler := handlers.NewMembershipTypeHandler(mtRepo, termsRepo)
	termsHandler := handlers.NewTermsHandler(termsRepo)
	redirectFlowRepo := repository.NewRedirectFlowSessionRepo(db)
	bankDetailsRepo := repository.NewBankDetailsRepo(db)
	_ = bankDetailsRepo // used later
	clientHandler := handlers.NewClientHandler(clientRepo, memberRepo, mtRepo, credRepo, redirectFlowRepo, enc, emailSvc)
	agreementRepo := repository.NewAgreementSnapshotRepo(db)
	signupHandler := handlers.NewSignupHandler(tenantRepo, mtRepo, bonusRepo, termsRepo, clientRepo, memberRepo, settingsRepo, credRepo, platformBillingConfigRepo, bankDetailsRepo, agreementRepo, tenantEmailTemplateRepo, enc, emailSvc, cfg)
	agreementHandler := handlers.NewAgreementHandler(agreementRepo)
	emailVerifier := handlers.NewEmailVerifyHandler(db, emailSvc, cfg.AppURL)
	registrationHandler := handlers.NewTenantRegistrationHandler(tenantRepo, userRepo, jwtMgr, sessionMgr, emailSvc, emailVerifier, cfg.AppURL)
	passwordResetHandler := handlers.NewPasswordResetHandler(userRepo, db, emailSvc, cfg.AppURL)
	paymentHandler := handlers.NewPaymentHandler(paymentRepo, db)
	reportsHandler := handlers.NewReportsHandler(reportsRepo)
	webhookHandler := handlers.NewWebhookHandler(webhookRepo, enc)
	inboundWebhookKeyHandler := handlers.NewInboundWebhookKeyHandler(inboundWebhookKeyRepo, enc)
	integrationsHandler := handlers.NewIntegrationsHandler(credRepo, platformBillingConfigRepo, enc)
	serviceRepo := repository.NewServiceRepo(db)
	gocardlessHandler := handlers.NewGoCardlessHandler(cfg, tenantRepo, credRepo, inboundWebhookEventRepo, paymentRepo, paymentDisputeRepo, memberRepo, mtRepo, clientRepo, tenantEmailTemplateRepo, enc, emailSvc)
	stripeWebhookHandler := handlers.NewStripeWebhookHandler(db, tenantRepo, credRepo, inboundWebhookEventRepo, enc, paymentRepo, paymentDisputeRepo, memberRepo, serviceRepo)
	inboundWebhookHandler := handlers.NewInboundWebhookHandler(clientRepo, inboundWebhookKeyRepo, inboundWebhookEventRepo, enc)
	activeMembershipsHandler := handlers.NewActiveMembershipsHandler(db, memberRepo, mtRepo, clientRepo, credRepo, enc)
	serviceHandler := handlers.NewServiceHandler(serviceRepo, clientRepo, memberRepo, mtRepo, paymentRepo, credRepo, enc)
	fileStore := storage.NewFileStore("./uploads")
	fileHandler := handlers.NewFileHandler(fileStore, db)
	dispatcher := events.NewDispatcher(db, rdb, enc)
	clientHandler.SetDispatcher(dispatcher)
	signupHandler.SetDispatcher(dispatcher)
	activeMembershipsHandler.SetDispatcher(dispatcher)
	gocardlessHandler.SetDispatcher(dispatcher)
	stripeWebhookHandler.SetDispatcher(dispatcher)

	// Health check
	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		metrics := observability.PaymentLifecycleMetrics.Snapshot()
		fmt.Fprintf(
			w,
			`{"status":"ok","service":"clientroll","payment_metrics":{"webhook_received":%d,"webhook_rejected":%d,"webhook_duplicate":%d,"webhook_processed":%d,"webhook_failed":%d,"payment_reconciled":%d}}`,
			metrics.WebhookReceived,
			metrics.WebhookRejected,
			metrics.WebhookDuplicate,
			metrics.WebhookProcessed,
			metrics.WebhookFailed,
			metrics.PaymentReconciled,
		)
	})

	// Public routes
	r.Route("/api", func(r chi.Router) {
		// Login (strict rate limit — 10/15min for brute force protection)
		r.Group(func(r chi.Router) {
			r.Use(loginLimiter.Limit)
			authHandler.RegisterRoutes(r)
		})

		// Public files (signup images, no auth)
		fileHandler.RegisterPublicRoutes(r)

		// Public signup flow (no auth required)
		signupHandler.RegisterPublicRoutes(r)

		// GoCardless OAuth callback and webhook (no auth)
		gocardlessHandler.RegisterPublicRoutes(r)
		stripeWebhookHandler.RegisterPublicRoutes(r)
		platformBillingWebhookHandler.RegisterPublicRoutes(r)
		inboundWebhookHandler.RegisterPublicRoutes(r)

		// Public slug check (no rate limit — read-only)
		r.Get("/register/check-slug/{slug}", registrationHandler.CheckSlug)

		// Registration + password reset + email verify (generous limit — 120/15min)
		r.Group(func(r chi.Router) {
			r.Use(rateLimiter.Limit)
			r.Post("/register", registrationHandler.Register)
			passwordResetHandler.RegisterRoutes(r)
			emailVerifier.RegisterRoutes(r)
		})

		// Authenticated routes
		r.Group(func(r chi.Router) {
			r.Use(authMW.RequireAuth)
			r.Use(tenantMW.SetTenantContext)
			r.Use(auditMW.LogWrites)

			// Current user (works for both tenant and platform admin sessions)
			r.Get("/me", func(w http.ResponseWriter, r *http.Request) {
				userID := middleware.GetUserID(r.Context())
				tenantIDStr := middleware.GetTenantID(r.Context())
				role := middleware.GetRole(r.Context())

				var setupCompletedAt *string
				if tid, err := uuid.Parse(tenantIDStr); err == nil && tid != uuid.Nil {
					if tenant, err := tenantRepo.FindByID(r.Context(), tid); err == nil && tenant != nil && tenant.SetupCompletedAt != nil {
						ts := tenant.SetupCompletedAt.Format("2006-01-02T15:04:05Z07:00")
						setupCompletedAt = &ts
					}
				}

				resp := map[string]interface{}{
					"user_id":            userID,
					"tenant_id":          tenantIDStr,
					"role":               role,
					"setup_completed_at": setupCompletedAt,
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				if err := json.NewEncoder(w).Encode(resp); err != nil {
					log.Printf("Error encoding /me response: %v", err)
				}
			})

			// Tenant-scoped routes (require tenant context)
			r.Route("/tenant", func(r chi.Router) {
				r.Use(tenantMW.RequireTenant)

				// Current user
				r.Get("/me", func(w http.ResponseWriter, r *http.Request) {
					userID := middleware.GetUserID(r.Context())
					tenantIDStr := middleware.GetTenantID(r.Context())
					role := middleware.GetRole(r.Context())

					var setupCompletedAt *string
					if tid, err := uuid.Parse(tenantIDStr); err == nil && tid != uuid.Nil {
						if tenant, err := tenantRepo.FindByID(r.Context(), tid); err == nil && tenant != nil && tenant.SetupCompletedAt != nil {
							ts := tenant.SetupCompletedAt.Format("2006-01-02T15:04:05Z07:00")
							setupCompletedAt = &ts
						}
					}

					resp := map[string]interface{}{
						"user_id":            userID,
						"tenant_id":          tenantIDStr,
						"role":               role,
						"setup_completed_at": setupCompletedAt,
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					if err := json.NewEncoder(w).Encode(resp); err != nil {
						log.Printf("Error encoding /tenant/me response: %v", err)
					}
				})

				// Settings (Phase 3)
				settingsHandler.RegisterRoutes(r)

				// Staff management (Phase 3)
				staffHandler.RegisterRoutes(r)

				// Core entities (Phase 4)
				bonusHandler.RegisterRoutes(r)
				mtHandler.RegisterRoutes(r)
				termsHandler.RegisterRoutes(r)
				clientHandler.RegisterRoutes(r)

				// Payments + Reports (Phase 6)
				paymentHandler.RegisterRoutes(r)
				reportsHandler.RegisterRoutes(r)

				// Active memberships view
				activeMembershipsHandler.RegisterRoutes(r)

				// Agreement downloads
				agreementHandler.RegisterRoutes(r)

				// Services (one-off charges)
				serviceHandler.RegisterRoutes(r)

				// Integrations (GoCardless, MailerSend keys)
				integrationsHandler.RegisterRoutes(r)
				// GoCardless Connect (OAuth start URL)
				gocardlessHandler.RegisterTenantRoutes(r)

				// Webhook management (Phase 7)
				webhookHandler.RegisterRoutes(r)
				inboundWebhookKeyHandler.RegisterRoutes(r)

				// File uploads + serving
				fileHandler.RegisterAuthRoutes(r)
			})

			// Platform admin routes
			r.Route("/platform", func(r chi.Router) {
				r.Use(authMW.RequirePlatformAdmin)
				platformHandler.RegisterRoutes(r)
				platformBillingHandler.RegisterRoutes(r)
				// Phase 8: Platform billing
			})
		})
	})

	return r
}
