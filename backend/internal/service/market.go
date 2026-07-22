package service

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/moduforge/backend/internal/domain"
)

type MarketService struct {
	mu      sync.RWMutex
	modules map[string]*domain.MarketModule
	reviews map[string][]*domain.MarketReview
	nextID  int
}

func NewMarketService() *MarketService {
	s := &MarketService{
		modules: make(map[string]*domain.MarketModule),
		reviews: make(map[string][]*domain.MarketReview),
	}
	s.seedData()
	return s
}

func (s *MarketService) seedData() {
	seeds := []domain.MarketModule{
		{Title: "System Prop Tweaks", Slug: "system-prop-tweaks", Description: "Comprehensive system property modifications for performance and battery optimization. Includes GPU, memory, and network tweaks.", Category: "system", Tags: "system,prop,performance,battery,gpu", Version: "v2.1", VersionCode: 5, Author: "ModuForge Team", License: "MIT", Stars: 128, Installs: 3500},
		{Title: "Custom Boot Animation", Slug: "boot-animation", Description: "Replace default boot animation with custom designs. Supports MP4 and PNG sequences.", Category: "ui", Tags: "boot,animation,custom,ui", Version: "v1.3", VersionCode: 3, Author: "DevMaster", License: "Apache-2.0", Stars: 89, Installs: 2100},
		{Title: "Audio Enhancement", Slug: "audio-enhance", Description: "Improve audio quality with custom DAC configurations and equalizer presets for popular headphones.", Category: "audio", Tags: "audio,dac,equalizer,enhance", Version: "v1.8", VersionCode: 7, Author: "SoundModder", License: "GPL-3.0", Stars: 156, Installs: 4200},
		{Title: "GPU Overclock Pro", Slug: "gpu-overclock", Description: "Safe GPU frequency adjustments for better gaming performance. Includes thermal monitoring.", Category: "display", Tags: "gpu,overclock,gaming,performance", Version: "v1.5", VersionCode: 4, Author: "GameTuner", License: "MIT", Stars: 234, Installs: 5800},
		{Title: "Network Firewall", Slug: "network-firewall", Description: "Per-app network access control with built-in ad blocking and DNS filtering.", Category: "utility", Tags: "network,firewall,adblock,dns,privacy", Version: "v2.0", VersionCode: 8, Author: "PrivacyGuard", License: "GPL-3.0", Stars: 312, Installs: 7600},
		{Title: "Battery Saver Max", Slug: "battery-saver", Description: "Intelligent battery management with Doze optimization and background process limiter.", Category: "system", Tags: "battery,doze,performance,optimization", Version: "v1.4", VersionCode: 6, Author: "BatteryPro", License: "MIT", Stars: 198, Installs: 4500},
		{Title: "Display Calibrator", Slug: "display-calibrate", Description: "Professional display calibration with ICC profiles and color temperature adjustments.", Category: "display", Tags: "display,calibrate,color,icc", Version: "v1.2", VersionCode: 2, Author: "ColorExpert", License: "MIT", Stars: 76, Installs: 1800},
		{Title: "Hosts AdBlock", Slug: "hosts-adblock", Description: "Hosts file based ad blocker with auto-update from multiple filter lists. 2M+ blocked domains.", Category: "utility", Tags: "adblock,hosts,privacy,network", Version: "v3.0", VersionCode: 12, Author: "AdGuardFork", License: "GPL-3.0", Stars: 456, Installs: 12000},
		{Title: "Magisk Manager Lite", Slug: "magisk-lite", Description: "Lightweight Magisk module management alternative with minimal UI overhead.", Category: "system", Tags: "magisk,manager,lite,system", Version: "v1.1", VersionCode: 1, Author: "LiteDev", License: "Apache-2.0", Stars: 45, Installs: 900},
		{Title: "Notification Sound Pack", Slug: "notification-sounds", Description: "Collection of 50+ notification sounds organized by category. Replace system notification tones.", Category: "ui", Tags: "notification,sounds,ringtones,ui", Version: "v1.6", VersionCode: 4, Author: "SoundPack", License: "CC-BY-4.0", Stars: 67, Installs: 2300},
	}

	for i := range seeds {
		s.nextID++
		seeds[i].ID = fmt.Sprintf("mod_%04d", s.nextID)
		seeds[i].CreatedAt = time.Now().AddDate(0, 0, -30-i)
		seeds[i].UpdatedAt = seeds[i].CreatedAt
		s.modules[seeds[i].ID] = &seeds[i]
	}
}

func (s *MarketService) ListModules(query, category, sortBy string, page, perPage int) ([]*domain.MarketModule, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var filtered []*domain.MarketModule
	for _, m := range s.modules {
		if category != "" && m.Category != category {
			continue
		}
		if query != "" {
			q := strings.ToLower(query)
			if !strings.Contains(strings.ToLower(m.Title), q) &&
				!strings.Contains(strings.ToLower(m.Description), q) &&
				!strings.Contains(strings.ToLower(m.Tags), q) {
				continue
			}
		}
		filtered = append(filtered, m)
	}

	switch sortBy {
	case "installs":
		sort.Slice(filtered, func(i, j int) bool { return filtered[i].Installs > filtered[j].Installs })
	case "newest":
		sort.Slice(filtered, func(i, j int) bool { return filtered[i].CreatedAt.After(filtered[j].CreatedAt) })
	default:
		sort.Slice(filtered, func(i, j int) bool { return filtered[i].Stars > filtered[j].Stars })
	}

	total := len(filtered)
	start := (page - 1) * perPage
	if start >= total {
		return nil, total
	}
	end := start + perPage
	if end > total {
		end = total
	}
	return filtered[start:end], total
}

func (s *MarketService) GetModule(slugOrID string) (*domain.MarketModule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.modules {
		if m.Slug == slugOrID || m.ID == slugOrID {
			return m, nil
		}
	}
	return nil, fmt.Errorf("module not found")
}

func (s *MarketService) StarModule(slugOrID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.modules {
		if m.Slug == slugOrID || m.ID == slugOrID {
			m.Stars++
			return m.Stars, nil
		}
	}
	return 0, fmt.Errorf("module not found")
}

func (s *MarketService) AddReview(moduleID, uid, username string, rating int, comment string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rating < 1 || rating > 5 {
		return fmt.Errorf("rating must be 1-5")
	}
	review := &domain.MarketReview{
		ID:        fmt.Sprintf("rev_%d", time.Now().UnixNano()),
		ModuleID:  moduleID,
		UID:       uid,
		Username:  username,
		Rating:    rating,
		Comment:   comment,
		CreatedAt: time.Now(),
	}
	s.reviews[moduleID] = append(s.reviews[moduleID], review)
	return nil
}

func (s *MarketService) GetReviews(moduleID string) []*domain.MarketReview {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.reviews[moduleID]
}

func (s *MarketService) PublishModule(mod *domain.MarketModule) (*domain.MarketModule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	mod.ID = fmt.Sprintf("mod_%04d", s.nextID)
	if mod.Slug == "" {
		mod.Slug = strings.ToLower(strings.ReplaceAll(mod.Title, " ", "-"))
	}
	mod.CreatedAt = time.Now()
	mod.UpdatedAt = time.Now()
	s.modules[mod.ID] = mod
	return mod, nil
}

func (s *MarketService) Categories() []string {
	return []string{"system", "ui", "audio", "display", "utility"}
}

func (s *MarketService) TrendingModules(limit int) []*domain.MarketModule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var trending []*domain.MarketModule
	for _, m := range s.modules {
		if m.Stars > 100 {
			trending = append(trending, m)
		}
	}
	sort.Slice(trending, func(i, j int) bool { return trending[i].Stars > trending[j].Stars })
	if limit > 0 && len(trending) > limit {
		trending = trending[:limit]
	}
	return trending
}
