package repository

import (
	"fmt"
	"time"
)

// TimeRangeType 时间范围类型
type TimeRangeType string

const (
	TimeRangeToday      TimeRangeType = "today"          // 今天
	TimeRangeYesterday  TimeRangeType = "yesterday"      // 昨天
	TimeRangeThisWeek   TimeRangeType = "this_week"      // 本周
	TimeRangeLastWeek   TimeRangeType = "last_week"      // 上周
	TimeRangeThisMonth  TimeRangeType = "this_month"     // 本月
	TimeRangeLastMonth  TimeRangeType = "last_month"     // 上个月
	TimeRangeLast7Days  TimeRangeType = "last_7_days"    // 过去七天
	TimeRangeLast30Days TimeRangeType = "last_30_days"   // 过去30天
	TimeRangeCustom     TimeRangeType = "custom"          // 自定义时间范围
)

// ParseTimeRange 解析时间范围
// 如果 timeRangeType 是 custom，则使用 startTimeStr 和 endTimeStr
// 如果只提供了日期没有时间，开始时间设为 00:00:00，结束时间设为 23:59:59
func ParseTimeRange(timeRangeType TimeRangeType, startTimeStr, endTimeStr string) (startTime, endTime time.Time, err error) {
	now := time.Now()
	year := now.Year()
	location := now.Location()

	switch timeRangeType {
	case TimeRangeToday:
		startTime = time.Date(year, now.Month(), now.Day(), 0, 0, 0, 0, location)
		endTime = time.Date(year, now.Month(), now.Day(), 23, 59, 59, 999999999, location)

	case TimeRangeYesterday:
		yesterday := now.AddDate(0, 0, -1)
		startTime = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, location)
		endTime = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 23, 59, 59, 999999999, location)

	case TimeRangeThisWeek:
		// 本周：周一到周日
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7 // 周日算作第7天
		}
		daysFromMonday := weekday - 1
		monday := now.AddDate(0, 0, -daysFromMonday)
		startTime = time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, location)
		sunday := monday.AddDate(0, 0, 6)
		endTime = time.Date(sunday.Year(), sunday.Month(), sunday.Day(), 23, 59, 59, 999999999, location)

	case TimeRangeLastWeek:
		// 上周：上周一到上周日
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		daysFromMonday := weekday - 1
		thisMonday := now.AddDate(0, 0, -daysFromMonday)
		lastMonday := thisMonday.AddDate(0, 0, -7)
		startTime = time.Date(lastMonday.Year(), lastMonday.Month(), lastMonday.Day(), 0, 0, 0, 0, location)
		lastSunday := lastMonday.AddDate(0, 0, 6)
		endTime = time.Date(lastSunday.Year(), lastSunday.Month(), lastSunday.Day(), 23, 59, 59, 999999999, location)

	case TimeRangeThisMonth:
		startTime = time.Date(year, now.Month(), 1, 0, 0, 0, 0, location)
		nextMonth := now.AddDate(0, 1, 0)
		endTime = time.Date(nextMonth.Year(), nextMonth.Month(), 1, 0, 0, 0, 0, location).Add(-time.Nanosecond)

	case TimeRangeLastMonth:
		lastMonth := now.AddDate(0, -1, 0)
		startTime = time.Date(lastMonth.Year(), lastMonth.Month(), 1, 0, 0, 0, 0, location)
		endTime = time.Date(year, now.Month(), 1, 0, 0, 0, 0, location).Add(-time.Nanosecond)

	case TimeRangeLast7Days:
		startTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location).AddDate(0, 0, -6)
		endTime = time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, location)

	case TimeRangeLast30Days:
		startTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location).AddDate(0, 0, -29)
		endTime = time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, location)

	case TimeRangeCustom:
		if startTimeStr == "" || endTimeStr == "" {
			return time.Time{}, time.Time{}, fmt.Errorf("custom time range requires both start_time and end_time")
		}

		// 尝试解析完整的时间格式 YYYY-MM-DD hh:mm:ss
		startTime, err = time.Parse("2006-01-02 15:04:05", startTimeStr)
		if err != nil {
			// 如果失败，尝试只解析日期 YYYY-MM-DD，然后设置为 00:00:00
			startTime, err = time.Parse("2006-01-02", startTimeStr)
			if err != nil {
				return time.Time{}, time.Time{}, fmt.Errorf("invalid start_time format: %v", err)
			}
			startTime = time.Date(startTime.Year(), startTime.Month(), startTime.Day(), 0, 0, 0, 0, location)
		}

		endTime, err = time.Parse("2006-01-02 15:04:05", endTimeStr)
		if err != nil {
			// 如果失败，尝试只解析日期 YYYY-MM-DD，然后设置为 23:59:59
			endTime, err = time.Parse("2006-01-02", endTimeStr)
			if err != nil {
				return time.Time{}, time.Time{}, fmt.Errorf("invalid end_time format: %v", err)
			}
			endTime = time.Date(endTime.Year(), endTime.Month(), endTime.Day(), 23, 59, 59, 999999999, location)
		}

	default:
		return time.Time{}, time.Time{}, fmt.Errorf("unknown time range type: %s", timeRangeType)
	}

	return startTime, endTime, nil
}

