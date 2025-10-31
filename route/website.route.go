package route

import (
	"github.com/MishraShardendu22/Scanner/controller"
	"github.com/MishraShardendu22/Scanner/templ_ms22"
	"github.com/gofiber/fiber/v2"
)

func RegisterWebRoutes(app *fiber.App) {
	app.Get("/robots.txt", func(c *fiber.Ctx) error {
		c.Set("Cache-Control", "public, max-age=86400")
		return c.SendFile("./public/robots.txt")
	})

	app.Get("/sitemap.xml", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "application/xml")
		c.Set("Cache-Control", "public, max-age=86400")
		return c.SendFile("./public/sitemap.xml")
	})

	app.Static("/public", "./public", fiber.Static{
		Compress:      true,
		ByteRange:     true,
		Browse:        false,
		CacheDuration: 24 * 60 * 60,
		MaxAge:        86400,
	})

	app.Get("/", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "text/html; charset=utf-8")
		c.Set("Cache-Control", "public, max-age=300")
		return templ_ms22.IndexNew().Render(c.Context(), c.Response().BodyWriter())
	})

	app.Get("/dashboard", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "text/html; charset=utf-8")
		c.Set("Cache-Control", "public, max-age=60")
		return templ_ms22.Dashboard().Render(c.Context(), c.Response().BodyWriter())
	})

	app.Get("/api/dashboard", controller.GetDashboardStats)

	app.Get("/scan", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "text/html; charset=utf-8")
		c.Set("Cache-Control", "public, max-age=600")
		return templ_ms22.ScanForm().Render(c.Context(), c.Response().BodyWriter())
	})

	app.Get("/results", controller.GetResultsPage)
	app.Get("/results/:request_id", controller.GetResultDetailPage)

	app.Get("/api-tester", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "text/html; charset=utf-8")
		c.Set("Cache-Control", "public, max-age=600")
		return templ_ms22.APITester().Render(c.Context(), c.Response().BodyWriter())
	})
}
