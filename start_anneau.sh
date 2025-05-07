#!/bin/bash

read -p "Combien d'instances voulez-vous créer ? " N

# Vérifie que l'entrée est un nombre positif et > 1
if ! [[ "$N" =~ ^[0-9]+$ ]] || [ "$N" -lt 2 ]; then
    echo "Erreur : vous devez entrer un entier supérieur ou égal à 2."
    exit 1
fi

# Crée le FIFO (tube nommé)
FIFO="/tmp/fifo_anneau"
RUN_COMMAND="go run main.go -n"

if [[ -p $FIFO ]]; then
    rm "$FIFO"
fi
mkfifo "$FIFO"

# Construction de la commande avec boucle
CMD="$RUN_COMMAND 0 < $FIFO"
for ((i=1; i<N; i++)); do
    CMD="$CMD | $RUN_COMMAND $i"
done
CMD="$CMD > $FIFO"

# Exécution de la commande dans un sous-shell pour éviter blocage stdin
bash -c "$CMD"
