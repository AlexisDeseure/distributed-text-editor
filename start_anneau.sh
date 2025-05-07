#!/bin/bash

RUN_COMMAND="go run main.go -n"

# Contourne l'erreur de Fyne
export LANG=en_US.UTF-8 

# Si un argument est fourni, on l'utilise, sinon on demande à l'utilisateur
if [ -n "$1" ]; then
    N="$1"
else
    read -p "Combien d'instances voulez-vous créer ? " N
fi

# Vérifie que l'entrée (argument ou saisie) est un entier ≥ 2
if ! [[ "$N" =~ ^[0-9]+$ ]] || [ "$N" -lt 2 ]; then
    echo "Erreur : vous devez entrer un entier supérieur ou égal à 2."
    exit 1
fi

# Crée le FIFO (tube nommé)
FIFO="/tmp/fifo_anneau"

if [[ -p $FIFO ]]; then
    rm "$FIFO"
fi
mkfifo "$FIFO"

# Construction de la commande en chaîne
CMD="$RUN_COMMAND 0 < $FIFO"
for ((i = 1; i < N; i++)); do
    CMD="$CMD | $RUN_COMMAND $i"
done
CMD="$CMD > $FIFO"

# Exécution de la commande dans un sous-shell pour éviter blocage stdin
bash -c "$CMD"
