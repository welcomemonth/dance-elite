package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	config "github.com/meetmonth/dance-elite/internal/configs"
	"github.com/meetmonth/dance-elite/internal/pkg/logger"
	"github.com/meetmonth/dance-elite/internal/pkg/storage"
	"github.com/meetmonth/dance-elite/internal/pkg/token"
	"github.com/meetmonth/dance-elite/internal/pkg/wechat"
	"github.com/meetmonth/dance-elite/internal/router"
	"github.com/meetmonth/dance-elite/internal/server"
	"github.com/meetmonth/dance-elite/internal/service"
	"github.com/meetmonth/dance-elite/internal/store"
	"github.com/meetmonth/dance-elite/internal/web"
)

// run 是组合根：在这里装配数据库、种子、路由与 HTTP 服务，并随 ctx 优雅退出。
// 返回错误由 main 统一记录，业务装配过程不 panic。
func run(ctx context.Context, cfg *config.Config) error {
	if dir := filepath.Dir(cfg.DBPath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("创建数据库目录: %w", err)
		}
	}

	db, err := store.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("打开数据库: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Log.Errorf("关闭数据库失败: %v", err)
		}
	}()

	if err := store.Migrate(ctx, db.SQL()); err != nil {
		return fmt.Errorf("数据库迁移: %w", err)
	}
	if err := db.Seed(ctx, store.SeedOptions{
		AdminUsername: cfg.AdminInitUsername,
		AdminPassword: cfg.AdminInitPassword,
	}); err != nil {
		return fmt.Errorf("写入种子数据: %w", err)
	}

	stores := store.New(db)
	tokens := token.NewSigner(cfg.JWTSecret, time.Duration(cfg.JWTExpireSeconds)*time.Second)
	wxClient := wechat.NewClient(cfg.WechatAppID, cfg.WechatSecret)
	authService := service.NewAuthService(stores, wxClient, tokens)
	adminAuthService := service.NewAdminAuthService(stores, tokens)
	seasonService := service.NewSeasonService(stores)
	eventService := service.NewEventService(stores)
	publicEventService := service.NewPublicEventService(stores)
	playerService := service.NewPlayerService(stores, cfg.IDCardHMACKey)
	registrationService := service.NewRegistrationService(stores, playerService, cfg.OrderExpireMinutes)
	registrationQueryService := service.NewRegistrationQueryService(stores)
	waitlistService := service.NewWaitlistService(stores, playerService)
	fileStore, err := storage.New(cfg.UploadDir)
	if err != nil {
		return fmt.Errorf("初始化上传目录: %w", err)
	}
	materialService := service.NewMaterialService(stores, fileStore)
	// 支付网关：PAY_MOCK=true 走 fake 通道（无商户证书可联调，R2）；
	// 否则加载商户私钥构造真实微信支付 V3 客户端，密钥缺失/解析失败在启动时报错。
	payGateway, err := newPayGateway(cfg)
	if err != nil {
		return err
	}
	paymentService := service.NewPaymentService(stores, payGateway, payGateway)
	refundService := service.NewRefundService(stores, payGateway, payGateway)
	orderService := service.NewOrderService(stores, payGateway, paymentService, cfg.OrderExpireMinutes)
	adminRefundQueryService := service.NewAdminRefundQueryService(stores)
	csTicketService := service.NewCSTicketService(stores)
	manualRegService := service.NewManualRegistrationService(stores, playerService)
	adminOrderQueryService := service.NewAdminOrderQueryService(stores)
	adminRegistrationQueryService := service.NewAdminRegistrationQueryService(stores)
	materialCollectionService := service.NewMaterialCollectionService(stores)
	dashboardService := service.NewDashboardService(stores)
	accountService := service.NewAdminAccountService(stores, tokens)
	adminLogQueryService := service.NewAdminLogQueryService(stores)
	userContentQueryService := service.NewAdminUserQueryService(stores)
	contentService := service.NewContentService(stores)
	audioReviewService := service.NewAudioReviewService(stores, cfg.PublicBaseURL)

	// 前端构建产物在编译期嵌入；未构建时（只有 .gitkeep）跳过页面托管，
	// 本地开发请使用 dance-admin 的 pnpm dev（代理到本服务）。
	webAssets, err := web.Dist()
	if err != nil {
		return fmt.Errorf("读取内嵌前端资源: %w", err)
	}
	if _, err := fs.Stat(webAssets, "index.html"); err != nil {
		logger.Log.Warn("前端构建产物未嵌入，后台页面不可用；发布前请在 dance-admin 执行 pnpm build")
		webAssets = nil
	}

	engine := router.New(router.Deps{
		Stores:              stores,
		Auth:                authService,
		AdminAuth:           adminAuthService,
		Seasons:             seasonService,
		Events:              eventService,
		PublicEvents:        publicEventService,
		Registrations:       registrationService,
		RegistrationQueries: registrationQueryService,
		Materials:           materialService,
		Orders:              orderService,
		Payments:            paymentService,
		Refunds:             refundService,
		AdminRefunds:        adminRefundQueryService,
		Waitlists:           waitlistService,
		CSTickets:           csTicketService,
		ManualRegs:          manualRegService,
		Players:             playerService,
		AdminOrders:         adminOrderQueryService,
		AdminRegs:           adminRegistrationQueryService,
		MaterialMissing:     materialCollectionService,
		Dashboard:           dashboardService,
		Accounts:            accountService,
		AdminLogs:           adminLogQueryService,
		UserContents:        userContentQueryService,
		Contents:            contentService,
		AudioReview:         audioReviewService,
		Tokens:              tokens,
		UploadDir:           cfg.UploadDir,
		WebAssets:           webAssets,
	})
	// 定时任务（T34）：应用级单调度器串行执行，ctx 取消后随 HTTP 服务一起退出。
	scheduler := service.NewStatusScheduler(
		time.Duration(cfg.StatusTickSeconds)*time.Second,
		service.NewEventStatusAdvancer(stores),
		service.NewOrderExpirySweeper(stores, paymentService, cfg.OrderExpireMinutes),
		service.NewRefundWatchdog(stores, refundService, payGateway, cfg.RefundWatchdogMinutes, cfg.PayMock),
	)
	schedulerCtx, stopScheduler := context.WithCancel(ctx)
	schedulerDone := make(chan struct{})
	go func() {
		defer close(schedulerDone)
		scheduler.Run(schedulerCtx)
	}()

	srv := server.NewServer(":"+cfg.Port, engine)
	runErr := srv.Run(ctx)
	// 等在途一轮定时任务结束（最多 5 秒），避免退出瞬间留下半轮写入。
	stopScheduler()
	select {
	case <-schedulerDone:
	case <-time.After(5 * time.Second):
		logger.Log.Warn("定时任务未在 5 秒内退出，进程即将结束")
	}
	if runErr != nil {
		return fmt.Errorf("HTTP 服务: %w", runErr)
	}
	return nil
}
