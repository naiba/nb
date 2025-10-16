package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/cloudflare/cloudflare-go"
	"github.com/samber/lo"

	"github.com/naiba/nb/assets"
)

type cloudflareServ struct {
	api        *cloudflare.API
	zones      []cloudflare.Zone
	zonesMap   map[string]cloudflare.Zone
	dnsRecords map[string][]cloudflare.DNSRecord
	pageRules  map[string][]cloudflare.PageRule
	rulesets   map[string][]cloudflare.Ruleset
}

var Cloudflared = &cloudflareServ{
	dnsRecords: make(map[string][]cloudflare.DNSRecord),
	pageRules:  make(map[string][]cloudflare.PageRule),
	rulesets:   make(map[string][]cloudflare.Ruleset),
}

func (cloudflareServ) Index(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(assets.CloudflareHtml))
}

func (s *cloudflareServ) State(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var apiToken string
	if s.api != nil {
		apiToken = s.api.APIToken
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"zones":      s.zones,
		"token":      apiToken,
		"dnsRecords": s.dnsRecords,
		"pageRules":  s.pageRules,
		"rulesets":   s.rulesets,
	})
}

func replaceDomainPlaceHolder(domainPrefix, domainSuffix, val string) string {
	val = strings.Replace(val, "#DOMAIN#", fmt.Sprintf("%s.%s", domainPrefix, domainSuffix), -1)
	val = strings.Replace(val, "#DOMAIN.PREFIX#", domainPrefix, -1)
	val = strings.Replace(val, "#DOMAIN.SUFFIX#", domainSuffix, -1)
	return val
}

func (s *cloudflareServ) BatchCreatePageRule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Zones []string `json:"zones"`
		Rule  struct {
			Priority int    `json:"priority"`
			Targets  string `json:"targets"`
			Actions  string `json:"actions"`
		} `json:"rule"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for _, zoneID := range req.Zones {
		lastIndex := strings.LastIndex(s.zonesMap[zoneID].Name, ".")
		domainPrefix, domainSuffix := s.zonesMap[zoneID].Name[:lastIndex], s.zonesMap[zoneID].Name[lastIndex+1:]

		targetsRaw := replaceDomainPlaceHolder(domainPrefix, domainSuffix, req.Rule.Targets)
		actionsRaw := replaceDomainPlaceHolder(domainPrefix, domainSuffix, req.Rule.Actions)

		var targets []cloudflare.PageRuleTarget
		if err := json.Unmarshal([]byte(targetsRaw), &targets); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var actions []cloudflare.PageRuleAction
		if err := json.Unmarshal([]byte(actionsRaw), &actions); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if _, err := s.api.CreatePageRule(r.Context(), zoneID, cloudflare.PageRule{
			Priority: req.Rule.Priority,
			Targets:  targets,
			Actions:  actions,
			Status:   "active",
		}); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	w.Write([]byte("ok"))
}

func (s *cloudflareServ) BatchCreateDNSRecord(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Zones  []string `json:"zones"`
		Record struct {
			Type    string `json:"type"`
			Name    string `json:"name"`
			Content string `json:"content"`
			TTL     int    `json:"ttl"`
			Proxied bool   `json:"proxied"`
		} `json:"record"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for _, zoneID := range req.Zones {
		lastIndex := strings.LastIndex(s.zonesMap[zoneID].Name, ".")
		domainPrefix, domainSuffix := s.zonesMap[zoneID].Name[:lastIndex], s.zonesMap[zoneID].Name[lastIndex+1:]
		if _, err := s.api.CreateDNSRecord(r.Context(), cloudflare.ZoneIdentifier(zoneID), cloudflare.CreateDNSRecordParams{
			Name:    replaceDomainPlaceHolder(domainPrefix, domainSuffix, req.Record.Name),
			Type:    req.Record.Type,
			Content: replaceDomainPlaceHolder(domainPrefix, domainSuffix, req.Record.Content),
			TTL:     req.Record.TTL,
			Proxied: &req.Record.Proxied,
		}); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	w.Write([]byte("ok"))
}

func (s *cloudflareServ) BatchCreateRuleset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Zones   []string `json:"zones"`
		Ruleset string
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for _, zoneID := range req.Zones {
		lastIndex := strings.LastIndex(s.zonesMap[zoneID].Name, ".")
		domainPrefix, domainSuffix := s.zonesMap[zoneID].Name[:lastIndex], s.zonesMap[zoneID].Name[lastIndex+1:]
		ruleset := replaceDomainPlaceHolder(domainPrefix, domainSuffix, req.Ruleset)
		var params cloudflare.CreateRulesetParams
		if err := json.Unmarshal([]byte(ruleset), &params); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if _, err := s.api.CreateRuleset(r.Context(), cloudflare.ZoneIdentifier(zoneID), params); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	w.Write([]byte("ok"))
}

func (s *cloudflareServ) LoadZoneRecords(w http.ResponseWriter, r *http.Request) {
	var zones []string
	if err := json.NewDecoder(r.Body).Decode(&zones); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for i := 0; i < len(zones); i++ {
		zoneID := zones[i]
		s.dnsRecords[zoneID] = make([]cloudflare.DNSRecord, 0)
		var j, totalPage = 1, 1
		for j <= totalPage {
			records, res, err := s.api.ListDNSRecords(r.Context(), cloudflare.ZoneIdentifier(zoneID), cloudflare.ListDNSRecordsParams{
				ResultInfo: cloudflare.ResultInfo{
					Page: j,
				},
			})
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if res != nil {
				totalPage = res.TotalPages
			}
			s.dnsRecords[zoneID] = append(s.dnsRecords[zoneID], records...)
			j++
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.dnsRecords)
}

func (s *cloudflareServ) LoadZonePageRules(w http.ResponseWriter, r *http.Request) {
	var zones []string
	if err := json.NewDecoder(r.Body).Decode(&zones); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for i := 0; i < len(zones); i++ {
		zoneID := zones[i]
		s.pageRules[zoneID] = make([]cloudflare.PageRule, 0)
		rules, err := s.api.ListPageRules(r.Context(), zoneID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.pageRules[zoneID] = rules
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.pageRules)
}

func (s *cloudflareServ) LoadZoneRulesets(w http.ResponseWriter, r *http.Request) {
	var zones []string
	if err := json.NewDecoder(r.Body).Decode(&zones); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for i := 0; i < len(zones); i++ {
		zoneID := zones[i]
		rulesets, err := s.api.ListRulesets(r.Context(), cloudflare.ZoneIdentifier(zoneID), cloudflare.ListRulesetsParams{})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.rulesets[zoneID] = lo.Filter(rulesets, func(ruleset cloudflare.Ruleset, index int) bool {
			return ruleset.Kind != "managed"
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.rulesets)
}

func (s *cloudflareServ) DeleteDNSRecords(w http.ResponseWriter, r *http.Request) {
	var records map[string][]string
	if err := json.NewDecoder(r.Body).Decode(&records); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for zoneID, ids := range records {
		for i := 0; i < len(ids); i++ {
			if err := s.api.DeleteDNSRecord(r.Context(), cloudflare.ZoneIdentifier(zoneID), ids[i]); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
	}
	w.Write([]byte("ok"))
}

func (s *cloudflareServ) DeletePageRules(w http.ResponseWriter, r *http.Request) {
	var records map[string][]string
	if err := json.NewDecoder(r.Body).Decode(&records); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for zoneID, ids := range records {
		for i := 0; i < len(ids); i++ {
			if err := s.api.DeletePageRule(r.Context(), zoneID, ids[i]); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
	}
	w.Write([]byte("ok"))
}

func (s *cloudflareServ) DeleteRulesets(w http.ResponseWriter, r *http.Request) {
	var records map[string][]string
	if err := json.NewDecoder(r.Body).Decode(&records); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for zoneID, ids := range records {
		for i := 0; i < len(ids); i++ {
			if err := s.api.DeleteRuleset(r.Context(), cloudflare.ZoneIdentifier(zoneID), ids[i]); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
	}
	w.Write([]byte("ok"))
}

func (s *cloudflareServ) CheckToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "ParseForm() err: "+err.Error(), http.StatusBadRequest)
		return
	}
	if r.Form.Get("token") == "" {
		http.Error(w, "Token is required", http.StatusBadRequest)
		return
	}
	var err error
	s.api, err = cloudflare.NewWithAPIToken(r.Form.Get("token"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.zones, err = s.api.ListZones(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.zonesMap = make(map[string]cloudflare.Zone)
	for i := 0; i < len(s.zones); i++ {
		s.zonesMap[s.zones[i].ID] = s.zones[i]
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"zones": s.zones,
		"token": s.api.APIToken,
	})
}
