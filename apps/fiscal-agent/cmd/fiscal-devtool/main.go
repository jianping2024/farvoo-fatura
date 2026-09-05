// Command fiscal-devtool: day-settle CLI for FS tip (PIN + keep amount).
// State (PIN / settled days) lives next to the exe JSON — not in fiscal.db.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"farvoo-fiscal-agent/internal/fiscal/store"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "settle":
		if err := runSettle(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "fiscal-devtool: %v\n", err)
			os.Exit(1)
		}
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "fiscal-devtool: unknown command %q\n", os.Args[1])
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `fiscal-devtool — settle FS tip for a business day (not shipped in Agent Setup)

  fiscal-devtool settle --db PATH --i-am-dev
      Interactive: PIN → date → keep amount → preview → confirm

  Non-interactive:
  fiscal-devtool settle --db PATH --i-am-dev --pin PIN [--new-pin PIN] \
      --date YYYY-MM-DD --keep AMOUNT --yes [--state PATH]

State file (PIN + settled days) defaults to <exeDir>/fiscal-devtool-state.json.
Stop Farvoo Fiscal Agent before opening fiscal.db.
`)
}

func runSettle(args []string) error {
	fs := flag.NewFlagSet("settle", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dbPath := fs.String("db", "", "path to fiscal.db")
	statePathFlag := fs.String("state", "", "tool state JSON (default: next to exe)")
	iAmDev := fs.Bool("i-am-dev", false, "required gate")
	pin := fs.String("pin", "", "PIN (omit for interactive prompt)")
	newPIN := fs.String("new-pin", "", "new PIN when must-change (non-interactive)")
	date := fs.String("date", "", "business date YYYY-MM-DD")
	keep := fs.String("keep", "", "keep amount e.g. 100.00")
	yes := fs.Bool("yes", false, "confirm without prompt")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !*iAmDev {
		return fmt.Errorf("refusing without --i-am-dev")
	}
	if strings.TrimSpace(*dbPath) == "" {
		return fmt.Errorf("--db is required")
	}
	statePath := strings.TrimSpace(*statePathFlag)
	if statePath == "" {
		p, err := DefaultStatePath()
		if err != nil {
			return err
		}
		statePath = p
	}

	toolState, err := LoadToolState(statePath)
	if err != nil {
		return err
	}

	db, err := store.Open(*dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w (is Agent still running?)", err)
	}
	defer db.Close()

	in := bufio.NewReader(os.Stdin)
	prompt := func(label string) (string, error) {
		fmt.Fprint(os.Stdout, label)
		line, err := in.ReadString('\n')
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(line), nil
	}

	entered := strings.TrimSpace(*pin)
	if entered == "" {
		entered, err = prompt("PIN: ")
		if err != nil {
			return err
		}
	}
	st, err := LoginPIN(statePath, entered)
	if err != nil {
		return err
	}
	if st.MustChangePIN {
		fmt.Fprintln(os.Stdout, "首次登录：必须修改 PIN（6 位数字）")
		np := strings.TrimSpace(*newPIN)
		if np == "" {
			np, err = prompt("新 PIN: ")
			if err != nil {
				return err
			}
			confirm, err := prompt("再输入一次: ")
			if err != nil {
				return err
			}
			if np != confirm {
				return fmt.Errorf("两次 PIN 不一致")
			}
		}
		if err := ChangePIN(statePath, entered, np); err != nil {
			return err
		}
		entered = np
		toolState, err = LoadToolState(statePath)
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, "PIN 已更新")
	}

	tz, err := LoadTaxpayerTimezone(db.SQL)
	if err != nil {
		return err
	}
	eligible, err := ListEligibleSettleDates(db.SQL, toolState, *dbPath, tz)
	if err != nil {
		return err
	}
	if len(eligible) == 0 {
		return fmt.Errorf("没有可结算的营业日（需有未结算的 FS，且该日为系列 tip）")
	}

	bizDate := strings.TrimSpace(*date)
	if bizDate == "" {
		def := DefaultSettleDate(eligible, tz)
		if *yes {
			bizDate = def
		} else {
			ans, err := prompt(fmt.Sprintf("营业日 [%s]: ", def))
			if err != nil {
				return err
			}
			if ans == "" {
				bizDate = def
			} else {
				bizDate = ans
			}
		}
	}
	if err := ValidateSettleDate(bizDate, eligible); err != nil {
		return err
	}

	keepStr := strings.TrimSpace(*keep)
	if keepStr == "" {
		keepStr, err = prompt("希望保留的金额: ")
		if err != nil {
			return err
		}
	}

	plan, err := BuildSettlePlan(context.Background(), db.SQL, SettlePlanInput{
		BusinessDate: bizDate,
		KeepTarget:   keepStr,
		DBPath:       *dbPath,
		State:        toolState,
	})
	if err != nil {
		return err
	}
	printPlan(plan)

	if !*yes {
		ans, err := prompt("确认结算？输入 yes: ")
		if err != nil {
			return err
		}
		if !strings.EqualFold(ans, "yes") {
			return fmt.Errorf("已取消")
		}
	}

	res, err := ApplySettlement(context.Background(), db.SQL, statePath, toolState, *dbPath, plan)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "已结算 %s：删除 %d 张，保留额累计 %s（目标 %s）\n状态: %s\n",
		res.BusinessDate, res.DeletedCount, res.KeepActual, res.KeepTarget, statePath)

	rep, err := db.VerifySeriesIntegrity(store.VerifySeriesIntegrityOptions{})
	if err != nil {
		return fmt.Errorf("integrity: %w", err)
	}
	if !rep.OK {
		return fmt.Errorf("settlement done but integrity failed (failed=%d)", rep.Failed)
	}
	fmt.Fprintln(os.Stdout, "系列完整性: OK")
	_ = entered
	return nil
}

func printPlan(p *SettlePlan) {
	fmt.Fprintln(os.Stdout, "—— 结算预览 ——")
	fmt.Fprintf(os.Stdout, "营业日: %s\n", p.BusinessDate)
	fmt.Fprintf(os.Stdout, "系列: %s (%s)\n", p.SeriesCode, p.SeriesID)
	fmt.Fprintf(os.Stdout, "保留目标: %s\n", p.KeepTarget)
	fmt.Fprintf(os.Stdout, "实际可保留（计入规则）: %s\n", p.KeepActual)
	if p.Shortfall {
		fmt.Fprintln(os.Stdout, "提示: 未凑满目标金额；允许 0 删除并仍可结算")
	}
	if p.AnchorInvoiceNo != "" {
		fmt.Fprintf(os.Stdout, "保护锚点（当日最后一张：非现金/真NIF/有NC·ND）: %s\n", p.AnchorInvoiceNo)
	} else {
		fmt.Fprintln(os.Stdout, "保护锚点: （无）从当日第一张起计保留额")
	}
	fmt.Fprintf(os.Stdout, "截止票（cutoff）: %s seq=%d\n", p.CutoffInvoiceNo, p.CutoffSeq)
	fmt.Fprintf(os.Stdout, "将删除: %d 张，总额 %s\n", len(p.DeleteIDs), p.DeleteGrossTotal)
	if len(p.DeleteInvoiceNos) > 0 {
		fmt.Fprintf(os.Stdout, "删除起点: %s\n", p.DeleteInvoiceNos[0])
		for _, no := range p.DeleteInvoiceNos {
			fmt.Fprintf(os.Stdout, "  - %s\n", no)
		}
	} else {
		fmt.Fprintln(os.Stdout, "删除起点: （无，0 删除）")
	}
}
