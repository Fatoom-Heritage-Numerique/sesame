// Commande sesame-fake : le contrat exécutable de Sésame (décision 0063). Elle
// assemble le code des décisions et les doubles d'internal/factice, lit la
// configuration du client à un chemin donné et sert, en mode grappe locale,
// les trois préfixes /n/{a|b|c}/canal, en mode instance le seul préfixe de
// sa configuration. Jamais une version de Sésame. Le lanceur factice exige
// sa variable (factice.LauncherEnv) à 1. Usage : sesame-fake -config <fichier>.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/factice"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/registre"
)

// reprise est la constante reprise de la version factice : celle d'une toute
// première version (ARCHITECTURE.md §6.4), registre vide à l'époque 0.
var reprise = &registre.Reprise{SansSauvegarde: true}

// maxConns borne les connexions ouvertes ensemble.
const maxConns = 1024

func main() {
	path := flag.String("config", "", "chemin de la configuration JSON du client")
	flag.Parse()
	if err := run(*path); err != nil {
		fmt.Fprintln(os.Stderr, "sesame-fake :", err)
		os.Exit(1)
	}
}

func run(path string) error {
	cfg, err := factice.ReadConfig(path)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	opt := factice.Options{Reprise: reprise}
	if cfg.PasserelleReelle != "" {
		opt.Passerelle = factice.HTTPGateway{Base: cfg.PasserelleReelle}
	}
	f, err := factice.New(ctx, cfg, opt)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", cfg.Ecoute)
	if err != nil {
		return err
	}
	srv := &http.Server{Handler: f.Server, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second,
		WriteTimeout: 30 * time.Second, IdleTimeout: 2 * time.Minute, MaxHeaderBytes: 16 << 10}
	go f.Run(ctx)
	go func() { <-ctx.Done(); _ = srv.Close() }()
	fmt.Fprintf(os.Stderr, "sesame-fake : %s, %d instance(s), sur %s\n", cfg.Mode, len(f.Server.Instances), ln.Addr())
	if err := srv.Serve(factice.LimitListener(ln, maxConns)); err != http.ErrServerClosed {
		return err
	}
	return nil
}
