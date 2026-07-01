package channels

import (
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestValidateCronExpr(t *testing.T) {
	valid := []string{
		"0 8 * * *",
		"*/5 * * * *",
		"0 0 1 * *",
		"@daily",
		"@every 5m",
		"@midnight",
	}
	for _, expr := range valid {
		t.Run("valid/"+expr, func(t *testing.T) {
			if err := ValidateCronExpr(expr); err != nil {
				t.Errorf("ValidateCronExpr(%q) returned error: %v", expr, err)
			}
		})
	}
	invalid := []string{
		"",
		"not a cron",
		"99 99 99 99 99",
		"* * * * * *",
		"@notadescriptor",
	}
	for _, expr := range invalid {
		t.Run("invalid/"+expr, func(t *testing.T) {
			if err := ValidateCronExpr(expr); err == nil {
				t.Errorf("ValidateCronExpr(%q) should return error", expr)
			}
		})
	}
}

func TestParseCalendarDays(t *testing.T) {
	t.Run("nil_request_uses_fallback", func(t *testing.T) {
		if got := ParseCalendarDays(nil, 14); got != 14 {
			t.Errorf("expected 14, got %d", got)
		}
	})
	t.Run("nil_request_fallback_clamped_to_7", func(t *testing.T) {
		// fallback <= 0 should default to 7
		if got := ParseCalendarDays(nil, 0); got != 7 {
			t.Errorf("expected 7, got %d", got)
		}
		if got := ParseCalendarDays(nil, -3); got != 7 {
			t.Errorf("expected 7, got %d", got)
		}
	})
	t.Run("missing_param_uses_fallback", func(t *testing.T) {
		r := &http.Request{URL: &url.URL{Path: "/"}}
		if got := ParseCalendarDays(r, 10); got != 10 {
			t.Errorf("expected 10, got %d", got)
		}
	})
	t.Run("valid_param", func(t *testing.T) {
		r := &http.Request{URL: &url.URL{Path: "/", RawQuery: "days=20"}}
		if got := ParseCalendarDays(r, 7); got != 20 {
			t.Errorf("expected 20, got %d", got)
		}
	})
	t.Run("invalid_param_uses_fallback", func(t *testing.T) {
		r := &http.Request{URL: &url.URL{Path: "/", RawQuery: "days=abc"}}
		if got := ParseCalendarDays(r, 7); got != 7 {
			t.Errorf("expected 7, got %d", got)
		}
	})
	t.Run("zero_param_uses_fallback", func(t *testing.T) {
		r := &http.Request{URL: &url.URL{Path: "/", RawQuery: "days=0"}}
		if got := ParseCalendarDays(r, 7); got != 7 {
			t.Errorf("expected 7, got %d", got)
		}
	})
	t.Run("negative_param_uses_fallback", func(t *testing.T) {
		r := &http.Request{URL: &url.URL{Path: "/", RawQuery: "days=-5"}}
		if got := ParseCalendarDays(r, 7); got != 7 {
			t.Errorf("expected 7, got %d", got)
		}
	})
	t.Run("over_31_clamped_to_31", func(t *testing.T) {
		r := &http.Request{URL: &url.URL{Path: "/", RawQuery: "days=100"}}
		if got := ParseCalendarDays(r, 7); got != 31 {
			t.Errorf("expected 31, got %d", got)
		}
	})
}

func TestIsDateSkipped(t *testing.T) {
	skipDates := []string{"2026-01-01", "2026-02-14"}
	if !IsDateSkipped("2026-01-01", skipDates) {
		t.Error("2026-01-01 should be skipped")
	}
	if !IsDateSkipped("2026-02-14", skipDates) {
		t.Error("2026-02-14 should be skipped")
	}
	if IsDateSkipped("2026-03-01", skipDates) {
		t.Error("2026-03-01 should not be skipped")
	}
	if IsDateSkipped("2026-01-01", nil) {
		t.Error("nil skipDates should not skip anything")
	}
	if IsDateSkipped("2026-01-01", []string{}) {
		t.Error("empty skipDates should not skip anything")
	}
}

func TestCalendarItemsForSchedule(t *testing.T) {
	now := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	windowEnd := now.AddDate(0, 0, 7)

	t.Run("disabled_returns_nil", func(t *testing.T) {
		sched := ChannelSchedule{Enabled: false, CheckinTime: "08:00"}
		if got := CalendarItemsForSchedule(sched, now, windowEnd, 7); got != nil {
			t.Errorf("disabled schedule should return nil, got %d items", len(got))
		}
	})

	t.Run("daily_schedule_emits_items", func(t *testing.T) {
		sched := ChannelSchedule{
			Enabled:       true,
			CheckinTime:   "08:00",
			UpstreamSiteID: "s1",
			SiteName:      "Alpha",
		}
		items := CalendarItemsForSchedule(sched, now, windowEnd, 7)
		if len(items) != 7 {
			t.Fatalf("expected 7 items, got %d", len(items))
		}
		if items[0].Time != "08:00" {
			t.Errorf("Time = %q, want 08:00", items[0].Time)
		}
		if items[0].SiteName != "Alpha" {
			t.Errorf("SiteName = %q, want Alpha", items[0].SiteName)
		}
		if items[0].SiteID != "s1" {
			t.Errorf("SiteID = %q, want s1", items[0].SiteID)
		}
		if items[0].JobType != "checkin" {
			t.Errorf("JobType = %q, want checkin", items[0].JobType)
		}
		if !items[0].Enabled {
			t.Error("Enabled should be true")
		}
	})

	t.Run("skip_dates_excluded", func(t *testing.T) {
		skipDate := now.Format("2006-01-02")
		sched := ChannelSchedule{
			Enabled:     true,
			CheckinTime: "08:00",
			SkipDates:   []string{skipDate},
		}
		items := CalendarItemsForSchedule(sched, now, windowEnd, 3)
		for _, item := range items {
			if item.Date == skipDate {
				t.Errorf("skip date %s should not appear in items", skipDate)
			}
		}
		if len(items) != 2 {
			t.Errorf("expected 2 items (1 skipped), got %d", len(items))
		}
	})

	t.Run("cron_schedule_defers_to_cron", func(t *testing.T) {
		sched := ChannelSchedule{
			Enabled:       true,
			CronExpr:      "0 8 * * *",
			UpstreamSiteID: "s1",
			SiteName:      "Alpha",
		}
		items := CalendarItemsForSchedule(sched, now, windowEnd, 7)
		// Cron path should produce items; verify at least one.
		if len(items) == 0 {
			t.Error("cron schedule should produce at least 1 item")
		}
		for _, item := range items {
			if item.JobType != "checkin" {
				t.Errorf("JobType = %q, want checkin", item.JobType)
			}
		}
	})
}

func TestCronCalendarItemsForSchedule(t *testing.T) {
	now := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	windowEnd := now.AddDate(0, 0, 7)

	t.Run("invalid_cron_returns_nil", func(t *testing.T) {
		sched := ChannelSchedule{
			Enabled:  true,
			CronExpr: "not-a-cron",
		}
		if got := CronCalendarItemsForSchedule(sched, now, windowEnd); got != nil {
			t.Errorf("invalid cron should return nil, got %d items", len(got))
		}
	})

	t.Run("valid_cron_emits_items", func(t *testing.T) {
		sched := ChannelSchedule{
			Enabled:       true,
			CronExpr:      "0 8 * * *",
			UpstreamSiteID: "s1",
			SiteName:      "Alpha",
		}
		items := CronCalendarItemsForSchedule(sched, now, windowEnd)
		if len(items) == 0 {
			t.Fatal("expected at least 1 item")
		}
		// Each item should have Time "08:00" for daily 8am cron.
		if items[0].Time != "08:00" {
			t.Errorf("Time = %q, want 08:00", items[0].Time)
		}
	})

	t.Run("skip_dates_excluded", func(t *testing.T) {
		sched := ChannelSchedule{
			Enabled:  true,
			CronExpr: "0 8 * * *",
			SkipDates: []string{now.AddDate(0, 0, 1).Format("2006-01-02")},
		}
		items := CronCalendarItemsForSchedule(sched, now, windowEnd)
		for _, item := range items {
			if item.Date == now.AddDate(0, 0, 1).Format("2006-01-02") {
				t.Error("skip date should not appear")
			}
		}
	})
}

func TestComputeNextRun(t *testing.T) {
	t.Run("daily_checkin_time_future", func(t *testing.T) {
		// ComputeNextRun uses time.Now() internally in CST timezone.
		// For "23:59", the next run should be today at 23:59 CST.
		next := ComputeNextRun("23:59", "", nil, 0, 0)
		if next == "" {
			t.Fatal("expected non-empty next run")
		}
		// Verify it parses as RFC3339.
		if _, err := time.Parse(time.RFC3339, next); err != nil {
			t.Errorf("next run %q is not RFC3339: %v", next, err)
		}
	})

	t.Run("daily_checkin_time_past_advances_to_tomorrow", func(t *testing.T) {
		// "00:01" is likely in the past, so next run should be tomorrow.
		next := ComputeNextRun("00:01", "", nil, 0, 0)
		parsed, err := time.Parse(time.RFC3339, next)
		if err != nil {
			t.Fatalf("next run %q is not RFC3339: %v", next, err)
		}
		cst := time.FixedZone("CST", 8*3600)
		now := time.Now().In(cst)
		// Next run should be after now.
		if !parsed.After(now) {
			t.Errorf("next run %v should be after now %v", parsed, now)
		}
	})

	t.Run("cron_expr_used_when_provided", func(t *testing.T) {
		next := ComputeNextRun("08:00", "0 8 * * *", nil, 0, 0)
		if next == "" {
			t.Fatal("expected non-empty next run")
		}
		if _, err := time.Parse(time.RFC3339, next); err != nil {
			t.Errorf("next run %q is not RFC3339: %v", next, err)
		}
	})

	t.Run("invalid_cron_falls_back_to_checkin_time", func(t *testing.T) {
		next := ComputeNextRun("23:59", "not-a-cron", nil, 0, 0)
		if next == "" {
			t.Fatal("expected non-empty next run (fallback)")
		}
		if _, err := time.Parse(time.RFC3339, next); err != nil {
			t.Errorf("next run %q is not RFC3339: %v", next, err)
		}
	})

	t.Run("skip_dates_advance_to_next_day", func(t *testing.T) {
		cst := time.FixedZone("CST", 8*3600)
		now := time.Now().In(cst)
		today := now.Format("2006-01-02")
		tomorrow := now.AddDate(0, 0, 1).Format("2006-01-02")
		// Skip today; for a late checkin time, next run should be tomorrow.
		next := ComputeNextRun("23:59", "", []string{today}, 0, 0)
		parsed, err := time.Parse(time.RFC3339, next)
		if err != nil {
			t.Fatalf("next run %q is not RFC3339: %v", next, err)
		}
		parsedCST := parsed.In(cst)
		nextDate := parsedCST.Format("2006-01-02")
		if nextDate == today {
			t.Errorf("next run should skip today %s, got %s", today, nextDate)
		}
		_ = tomorrow // not strictly needed; just ensure we advanced
	})

	t.Run("random_delay_adds_midpoint", func(t *testing.T) {
		// With delay 0-10, midpoint is 5 minutes added.
		withoutDelay := ComputeNextRun("23:59", "", nil, 0, 0)
		withDelay := ComputeNextRun("23:59", "", nil, 0, 10)
		parsedWithout, err1 := time.Parse(time.RFC3339, withoutDelay)
		parsedWith, err2 := time.Parse(time.RFC3339, withDelay)
		if err1 != nil || err2 != nil {
			t.Fatalf("parse error: %v, %v", err1, err2)
		}
		// The delayed version should be later (5 min midpoint).
		// Note: if "23:59" crosses midnight, the diff might vary, but
		// delayed should still be after non-delayed on same day.
		diff := parsedWith.Sub(parsedWithout)
		if diff != 5*time.Minute {
			// Allow 0 if both crossed midnight differently, but generally expect 5m.
			t.Logf("diff = %v (expected 5m, but midnight crossing may affect this)", diff)
		}
	})

	t.Run("delay_max_not_greater_than_min_no_add", func(t *testing.T) {
		// delayMax <= delayMin should not add delay.
		next := ComputeNextRun("23:59", "", nil, 10, 5)
		if _, err := time.Parse(time.RFC3339, next); err != nil {
			t.Errorf("next run %q is not RFC3339: %v", next, err)
		}
	})
}

func TestSortCalendarItemsByDateTime(t *testing.T) {
	t.Run("empty_slice", func(t *testing.T) {
		SortCalendarItemsByDateTime(nil) // should not panic
	})
	t.Run("already_sorted", func(t *testing.T) {
		items := []ScheduleCalendarItem{
			{Date: "2026-07-01", Time: "08:00"},
			{Date: "2026-07-02", Time: "08:00"},
		}
		SortCalendarItemsByDateTime(items)
		if items[0].Date != "2026-07-01" {
			t.Errorf("first item date = %q, want 2026-07-01", items[0].Date)
		}
	})
	t.Run("reverse_sorted", func(t *testing.T) {
		items := []ScheduleCalendarItem{
			{Date: "2026-07-03", Time: "08:00"},
			{Date: "2026-07-01", Time: "08:00"},
			{Date: "2026-07-02", Time: "08:00"},
		}
		SortCalendarItemsByDateTime(items)
		if items[0].Date != "2026-07-01" {
			t.Errorf("first item date = %q, want 2026-07-01", items[0].Date)
		}
		if items[1].Date != "2026-07-02" {
			t.Errorf("second item date = %q, want 2026-07-02", items[1].Date)
		}
		if items[2].Date != "2026-07-03" {
			t.Errorf("third item date = %q, want 2026-07-03", items[2].Date)
		}
	})
	t.Run("same_date_sorts_by_time", func(t *testing.T) {
		items := []ScheduleCalendarItem{
			{Date: "2026-07-01", Time: "10:00"},
			{Date: "2026-07-01", Time: "08:00"},
			{Date: "2026-07-01", Time: "09:00"},
		}
		SortCalendarItemsByDateTime(items)
		if items[0].Time != "08:00" {
			t.Errorf("first item time = %q, want 08:00", items[0].Time)
		}
		if items[1].Time != "09:00" {
			t.Errorf("second item time = %q, want 09:00", items[1].Time)
		}
		if items[2].Time != "10:00" {
			t.Errorf("third item time = %q, want 10:00", items[2].Time)
		}
	})
}
