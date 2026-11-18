package main

import (
	"github.com/go-chi/chi/v5"
)

// RegisterFinanceRoutes wires every personal-finance endpoint a logged-in
// tenant uses: /spending (CRUD + search/recent for bots is in bot.go),
// /budgeting/*, /assets/*, /setup, /categories, /accounts, /income-sources,
// /trend, /report/*, /patterns/*, /years*, /summary, /fx-rates. Ported 1:1
// from app.py's routes of the same names - see that file for the reference
// behavior/JSON shape. Every handler (except /fx-rates, public in Python
// too) starts with requireAccount() and every query is scoped by
// account_id=$N - never trust a path/body id alone.
func RegisterFinanceRoutes(r chi.Router, app *App) {
	r.With(publicCORS).Get("/fx-rates", app.handleFxRates) // public; CORS so packaged TV viewers can read it

	r.Get("/setup", app.handleGetSetup)

	r.Post("/categories", app.handleAddCategory)
	r.Patch("/categories/{cid}", app.handleUpdateCategory)
	r.Delete("/categories/{cid}", app.handleDeleteCategory)

	r.Post("/accounts", app.handleAddAccount)
	r.Patch("/accounts/{aid}", app.handleUpdateAccount)
	r.Delete("/accounts/{aid}", app.handleDeleteAccount)

	r.Post("/income-sources", app.handleAddIncomeSource)
	r.Patch("/income-sources/{sid}", app.handleUpdateIncomeSource)
	r.Delete("/income-sources/{sid}", app.handleDeleteIncomeSource)

	r.Get("/trend", app.handleTrend)
	r.Get("/years", app.handleYearsOverview)
	r.Get("/years/by-category", app.handleYearsByCategory)
	r.Get("/years/monthly", app.handleYearsMonthly)
	r.Get("/years/day-of-month-avg", app.handleYearsDayOfMonthAvg)
	r.Get("/years/datemonth-avg", app.handleYearsDatemonthAvg)

	r.Get("/report/weekly/{year}/{month}", app.handleWeeklyReport)

	r.Get("/budgeting/{year}", app.handleGetBudgeting)
	r.Post("/budgeting/income", app.handleSetBudgetIncome)
	r.Post("/budgeting/expense", app.handleSetBudgetExpense)

	r.Get("/spending", app.handleListSpending)
	r.Post("/spending", app.handleAddSpending)
	r.Delete("/spending/{sid}", app.handleDeleteSpending)
	r.Patch("/spending/{sid}", app.handleUpdateSpending)

	r.Get("/assets", app.handleGetAssets)
	r.Post("/assets", app.handleAddAsset)
	r.Patch("/assets/{aid}", app.handleUpdateAsset)
	r.Delete("/assets/{aid}", app.handleDeleteAsset)
	r.Post("/assets/target", app.handleSetAssetTarget)

	r.Get("/report/year/{year}", app.handleReportYear)
	r.Get("/report/month-compare/{year}/{month}", app.handleReportMonthCompare)
	r.Get("/report/{year}/{month}", app.handleReport)

	r.Get("/patterns/year/{year}", app.handlePatternsYear)

	r.Get("/summary", app.handleSummary)
}
