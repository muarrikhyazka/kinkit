package main

import (
	"context"
	"log"
	"time"

	"github.com/go-chi/chi/v5"
)

// RegisterMoneyRoutes wires recurring transactions, actual income entries,
// savings goals, budget actuals and budget-alert settings. All login
// required and scoped by account_id.
func RegisterMoneyRoutes(r chi.Router, app *App) {
	r.Get("/recurring", app.handleListRecurring)
	r.Post("/recurring", app.handleAddRecurring)
	r.Patch("/recurring/{id}", app.handleUpdateRecurring)
	r.Delete("/recurring/{id}", app.handleDeleteRecurring)

	r.Get("/income", app.handleListIncome)
	r.Post("/income", app.handleAddIncome)
	r.Patch("/income/{id}", app.handleUpdateIncome)
	r.Delete("/income/{id}", app.handleDeleteIncome)

	r.Get("/goals", app.handleListGoals)
	r.Post("/goals", app.handleAddGoal)
	r.Patch("/goals/{id}", app.handleUpdateGoal)
	r.Delete("/goals/{id}", app.handleDeleteGoal)
	r.Post("/goals/{id}/contribute", app.handleContributeGoal)

	r.Get("/budgeting/{year}/actuals", app.handleBudgetActuals)

	r.Get("/alerts/settings", app.handleGetAlertSettings)
	r.Patch("/alerts/settings", app.handleUpdateAlertSettings)
}

const (
	moneyAlertInterval     = 5 * time.Minute
	moneyRecurringInterval = time.Hour
)

// startMoneyWorker launches the background loop: recurring generation on
// start + hourly, budget alerts on start + every 5 minutes. Each pass is
// panic-safe and logs errors instead of crashing the server.
func (app *App) startMoneyWorker(ctx context.Context) {
	go func() {
		safe := func(name string, fn func(context.Context) error) {
			defer func() {
				if p := recover(); p != nil {
					log.Printf("money worker: %s panic: %v", name, p)
				}
			}()
			pctx, cancel := context.WithTimeout(ctx, 4*time.Minute)
			defer cancel()
			if err := fn(pctx); err != nil {
				log.Printf("money worker: %s: %v", name, err)
			}
		}
		recurring := func(c context.Context) error {
			n, err := app.generateRecurring(c, 0, 0, time.Now().UTC())
			if n > 0 {
				log.Printf("money worker: generated %d recurring transaction(s)", n)
			}
			return err
		}
		alerts := func(c context.Context) error { return app.checkBudgetAlerts(c, time.Now().UTC()) }

		safe("recurring", recurring)
		safe("alerts", alerts)
		lastRecurring := time.Now()
		t := time.NewTicker(moneyAlertInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if time.Since(lastRecurring) >= moneyRecurringInterval {
					safe("recurring", recurring)
					lastRecurring = time.Now()
				}
				safe("alerts", alerts)
			}
		}
	}()
}
