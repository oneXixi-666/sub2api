package service

// Ticket scope matches the mg harvester: only Astra and Sol are probed,
// regardless of other account mappings or extra configured models.
func openAICodexTicketModels(account *Account, configured []string) []string {
	if !isOpenAICodexTicketAccount(account) {
		return nil
	}
	_ = configured
	return []string{openAICodexTicketDefaultModel, openAICodexTicketDefaultSolModel}
}

func openAICodexTicketIncludesModel(account *Account, configured []string, model string) bool {
	model = normalizeOpenAICodexTicketModel(model)
	for _, candidate := range openAICodexTicketModels(account, configured) {
		if candidate == model {
			return true
		}
	}
	return false
}

func openAICodexTicketModelEligible(account *Account, model string) bool {
	return openAICodexTicketIncludesModel(account, nil, model)
}
