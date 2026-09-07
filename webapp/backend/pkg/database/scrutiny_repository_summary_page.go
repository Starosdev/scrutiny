package database

import (
	"context"
	"sort"
	"strings"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
)

const defaultDashboardPageSize = 25
const defaultDashboardHostPageSize = 10

var validDashboardPageSizes = map[int]struct{}{
	25:  {},
	50:  {},
	100: {},
	250: {},
}

var validDashboardHostPageSizes = map[int]struct{}{
	5:  {},
	10: {},
	25: {},
	50: {},
}

func (sr *scrutinyRepository) GetSummaryPage(ctx context.Context, options models.DeviceSummaryPageOptions) (*models.DeviceSummaryPage, error) {
	summaries, err := sr.getSummary(ctx, false)
	if err != nil {
		return nil, err
	}

	normalizeSummaryPageOptions(sr, &options)
	return buildSummaryPage(summaries, options), nil
}

func buildSummaryPage(summaries map[string]*models.DeviceSummary, options models.DeviceSummaryPageOptions) *models.DeviceSummaryPage {
	ordered := make([]*models.DeviceSummary, 0, len(summaries))
	attentionCount := 0
	hostSearch := strings.ToLower(strings.TrimSpace(options.HostSearch))
	for _, summary := range summaries {
		if !summary.Device.Archived && summary.Device.DeviceStatus > 0 {
			attentionCount++
		}
		if summary.Device.Archived == options.Archived && (hostSearch == "" || strings.Contains(strings.ToLower(summary.Device.HostId), hostSearch)) {
			ordered = append(ordered, summary)
		}
	}

	sort.SliceStable(ordered, func(i, j int) bool {
		return compareDeviceSummaries(ordered[i], ordered[j], options.Sort, options.Display) < 0
	})

	totalItems := len(ordered)
	if options.GroupByHost {
		hosts := make([]string, 0)
		seenHosts := make(map[string]struct{})
		for _, summary := range ordered {
			if _, seen := seenHosts[summary.Device.HostId]; seen {
				continue
			}
			seenHosts[summary.Device.HostId] = struct{}{}
			hosts = append(hosts, summary.Device.HostId)
		}

		totalItems = len(hosts)
		start, end := summaryPageBounds(totalItems, options.Page, options.PageSize)
		selectedHosts := make(map[string]struct{}, end-start)
		for _, host := range hosts[start:end] {
			selectedHosts[host] = struct{}{}
		}
		ordered = summariesForHosts(ordered, selectedHosts)
	}

	totalPages := 0
	if totalItems > 0 {
		totalPages = (totalItems + options.PageSize - 1) / options.PageSize
	}

	start, end := 0, len(ordered)
	if !options.GroupByHost {
		start, end = summaryPageBounds(totalItems, options.Page, options.PageSize)
	}

	pageSummaries := make(map[string]*models.DeviceSummary, end-start)
	for _, summary := range ordered[start:end] {
		pageSummaries[summary.Device.DeviceID] = summary
	}

	return &models.DeviceSummaryPage{
		Summary: pageSummaries,
		Pagination: models.PaginationMetadata{
			Page:           options.Page,
			PageSize:       options.PageSize,
			TotalItems:     totalItems,
			TotalPages:     totalPages,
			AttentionCount: attentionCount,
		},
	}
}

func summaryPageBounds(totalItems, page, pageSize int) (int, int) {
	start := (page - 1) * pageSize
	if start > totalItems {
		start = totalItems
	}
	end := start + pageSize
	if end > totalItems {
		end = totalItems
	}
	return start, end
}

func summariesForHosts(summaries []*models.DeviceSummary, hosts map[string]struct{}) []*models.DeviceSummary {
	selected := make([]*models.DeviceSummary, 0)
	for _, summary := range summaries {
		if _, ok := hosts[summary.Device.HostId]; ok {
			selected = append(selected, summary)
		}
	}
	return selected
}

func normalizeSummaryPageOptions(sr *scrutinyRepository, options *models.DeviceSummaryPageOptions) {
	if options.Page < 1 {
		options.Page = 1
	}
	validPageSizes := validDashboardPageSizes
	settingKey := "user.dashboard_page_size"
	defaultPageSize := defaultDashboardPageSize
	if options.GroupByHost {
		validPageSizes = validDashboardHostPageSizes
		settingKey = "user.dashboard_host_page_size"
		defaultPageSize = defaultDashboardHostPageSize
	}
	if _, ok := validPageSizes[options.PageSize]; !ok {
		options.PageSize = sr.appConfig.GetInt(settingKey)
	}
	if _, ok := validPageSizes[options.PageSize]; !ok {
		options.PageSize = defaultPageSize
	}
	if options.Sort == "" {
		options.Sort = sr.appConfig.GetString("user.dashboard_sort")
	}
	if options.Display == "" {
		options.Display = sr.appConfig.GetString("user.dashboard_display")
	}
}

func compareDeviceSummaries(left, right *models.DeviceSummary, sortBy, display string) int {
	if hostComparison := strings.Compare(left.Device.HostId, right.Device.HostId); hostComparison != 0 {
		return hostComparison
	}

	normalizedSort := normalizeDashboardSort(sortBy)
	descending := strings.HasSuffix(normalizedSort, "_desc")
	sortField := strings.TrimSuffix(strings.TrimSuffix(normalizedSort, "_asc"), "_desc")

	comparison := 0
	switch sortField {
	case "title":
		comparison = strings.Compare(deviceTitleForType(&left.Device, display), deviceTitleForType(&right.Device, display))
	case "age":
		comparison = compareInt64(summaryPowerOnHours(left), summaryPowerOnHours(right))
	case "capacity":
		comparison = compareInt64(left.Device.Capacity, right.Device.Capacity)
	case "temperature":
		comparison = compareInt64(summaryTemperature(left), summaryTemperature(right))
	default:
		comparison = compareInt64(summaryStatusValue(left), summaryStatusValue(right))
	}

	if descending {
		comparison *= -1
	}
	if comparison != 0 {
		return comparison
	}
	return strings.Compare(left.Device.DeviceID, right.Device.DeviceID)
}

func normalizeDashboardSort(sortBy string) string {
	switch sortBy {
	case "status":
		return "status_desc"
	case "title":
		return "title_asc"
	case "age":
		return "age_asc"
	case "":
		return "status_desc"
	default:
		return sortBy
	}
}

func summaryStatusValue(summary *models.DeviceSummary) int64 {
	if summary.SmartResults == nil {
		return 0
	}
	if summary.Device.DeviceStatus == 0 {
		return 1
	}
	return int64(summary.Device.DeviceStatus) * -1
}

func summaryPowerOnHours(summary *models.DeviceSummary) int64 {
	if summary.SmartResults == nil {
		return 0
	}
	return summary.SmartResults.PowerOnHours
}

func summaryTemperature(summary *models.DeviceSummary) int64 {
	if summary.SmartResults == nil || summary.SmartResults.Temp == nil {
		return int64(^uint64(0) >> 1)
	}
	return *summary.SmartResults.Temp
}

func compareInt64(left, right int64) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func deviceTitleForType(device *models.Device, display string) string {
	title := ""
	switch display {
	case "serial_id":
		if device.DeviceSerialID != "" {
			title = "/by-id/" + device.DeviceSerialID
		}
	case "uuid":
		if device.DeviceUUID != "" {
			title = "/by-uuid/" + device.DeviceUUID
		}
	case "label":
		switch {
		case device.Label != "":
			title = device.Label
		case device.DeviceLabel != "":
			title = "/by-label/" + device.DeviceLabel
		}
	default:
		title = deviceNameTitle(device)
	}
	if title == "" {
		title = deviceNameTitle(device)
	}
	return title
}

func deviceNameTitle(device *models.Device) string {
	parts := make([]string, 0, 3)
	if device.DeviceName != "" {
		if strings.HasPrefix(device.DeviceName, "/dev/") {
			parts = append(parts, device.DeviceName)
		} else {
			parts = append(parts, "/dev/"+device.DeviceName)
		}
	}
	if device.DeviceType != "" && device.DeviceType != "scsi" && device.DeviceType != "ata" {
		parts = append(parts, device.DeviceType)
	}
	if device.ModelName != "" {
		parts = append(parts, device.ModelName)
	}
	return strings.Join(parts, " - ")
}
