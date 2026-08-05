package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: go run ./cmd/openai-account-routes <list|set|delete> [flags]")
	}

	command := args[0]
	apiKeyID, accountID, err := parseIDs(command, args[1:])
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	routes := service.NewOpenAIAPIKeyAccountRouteStore("")

	switch command {
	case "list":
		values, err := routes.List(ctx)
		if err != nil {
			return err
		}
		ids := make([]int64, 0, len(values))
		for id := range values {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		for _, id := range ids {
			fmt.Printf("api_key_id=%d account_id=%d\n", id, values[id])
		}
		return nil
	case "set":
		if apiKeyID <= 0 || accountID <= 0 {
			return fmt.Errorf("set requires --api-key-id and --account-id")
		}
		if err := routes.Set(ctx, apiKeyID, accountID); err != nil {
			return err
		}
		fmt.Printf("set api_key_id=%d account_id=%d\n", apiKeyID, accountID)
		return nil
	case "delete":
		if apiKeyID <= 0 {
			return fmt.Errorf("delete requires --api-key-id")
		}
		if err := routes.Delete(ctx, apiKeyID); err != nil {
			return err
		}
		fmt.Printf("deleted api_key_id=%d\n", apiKeyID)
		return nil
	default:
		return fmt.Errorf("unknown command %q; use list, set, or delete", command)
	}
}

func parseIDs(command string, args []string) (int64, int64, error) {
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	apiKeyID := fs.String("api-key-id", "", "API key ID")
	accountID := fs.String("account-id", "", "OpenAI account ID")
	if err := fs.Parse(args); err != nil {
		return 0, 0, err
	}
	parse := func(name, raw string) (int64, error) {
		if raw == "" {
			return 0, nil
		}
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value <= 0 {
			return 0, fmt.Errorf("%s must be a positive integer", name)
		}
		return value, nil
	}
	keyID, err := parse("--api-key-id", *apiKeyID)
	if err != nil {
		return 0, 0, err
	}
	accID, err := parse("--account-id", *accountID)
	if err != nil {
		return 0, 0, err
	}
	return keyID, accID, nil
}
