// Commande sesame-fake : le contrat exécutable de Sésame (décision 0063). Elle
// assemble le code des décisions et les doubles d'internal/factice, lit la
// configuration du client à un chemin donné et sert, en mode grappe locale,
// les trois préfixes /n/{a|b|c}/canal, en mode instance le seul préfixe de
// sa configuration. Jamais une version de Sésame. Le lanceur factice exige
// sa variable (factice.LauncherEnv) à 1. Usage : sesame-fake -config <fichier>
// [-prefixe a|b|c -ecoute <adresse> -ecoute-grappe <adresse> -pair <vm>=<adresse>…] ;
// -prefixe passe en mode instance (grappe sur trois conteneurs), les autres
// options remplacent les champs ecoute, ecouteGrappe et pairs de la
// configuration.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Fatoom-Heritage-Numerique/sesame/internal/factice"
	"github.com/Fatoom-Heritage-Numerique/sesame/internal/registre"
)

// reprise est la constante reprise de la version factice : celle d'une toute
// première version (ARCHITECTURE.md §6.4), registre vide à l'époque 0.
var reprise = &registre.Reprise{SansSauvegarde: true}

// over sont les options qui remplacent la configuration.
var over = struct {
	prefixe, ecoute, grappe string
	pairs                   map[string]string
}{pairs: map[string]string{}}

// maxConns borne les connexions ouvertes ensemble.
const maxConns = 1024

func main() {
	path := flag.String("config", "", "chemin de la configuration JSON du client")
	flag.StringVar(&over.prefixe, "prefixe", "", "mode instance : préfixe de routage de cette instance")
	flag.StringVar(&over.ecoute, "ecoute", "", "adresse d'écoute des clients")
	flag.StringVar(&over.grappe, "ecoute-grappe", "", "adresse d'écoute des routes entre VM (mode instance)")
	flag.Func("pair", "autre instance, <nom de VM>=<adresse http> (mode instance, répétable)", func(v string) error {
		k, u, ok := strings.Cut(v, "=")
		if !ok {
			return fmt.Errorf("pair attendu sous la forme <vm>=<adresse>")
		}
		over.pairs[k] = u
		return nil
	})
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
	if over.prefixe != "" {
		cfg.Mode, cfg.Prefixe = "instance", over.prefixe
	}
	if over.ecoute != "" {
		cfg.Ecoute = over.ecoute
	}
	if over.grappe != "" {
		cfg.EcouteGrappe = over.grappe
	}
	if len(over.pairs) > 0 {
		cfg.Pairs = over.pairs
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
	if cfg.Mode == "instance" && cfg.EcouteGrappe != "" {
		g := &http.Server{Addr: cfg.EcouteGrappe, Handler: factice.GrappeHandler(f.Server.Instances[0]), ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
		go func() { <-ctx.Done(); _ = g.Close() }()
		go func() { _ = g.ListenAndServe() }()
	}
	go func() { <-ctx.Done(); _ = srv.Close() }()
	fmt.Fprintf(os.Stderr, "sesame-fake : %s, %d instance(s), sur %s\n", cfg.Mode, len(f.Server.Instances), ln.Addr())
	if err := srv.Serve(factice.LimitListener(ln, maxConns)); err != http.ErrServerClosed {
		return err
	}
	return nil
}
