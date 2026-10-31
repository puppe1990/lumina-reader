package handlers

import (
	"time"

	"github.com/puppe1990/amarra-cais/pkg/cais/i18n"

	appi18n "github.com/puppe1990/lumina/internal/i18n"
)

func appCatalog() *i18n.Catalog { return appi18n.DefaultCatalog() }

func testClock(iso string) func() time.Time {
	quando, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		panic(err)
	}
	return func() time.Time { return quando }
}
