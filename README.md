# *Project SR05 Groupe 12* - Application répartie pour un système de traitement textuel décentralisé

Projet réalisé par : 
* Alexis Deseure--Charron
* Antoine Lequeux
* Jessica Devulder
* Timo Allais

## Description du projet

Ce projet a pour but de créer une application distribuée permettant de traiter un fichier texte de manière décentralisée. Cette application comporte des sites reliés en réseau (anneau) entre eux afin de se partager les modifications du fichier texte. L'implémentation de l'algorithme de la file d'attente répartie permet de gérer la concordance entre les différents sites.

## Exécution

Pour exécuter le projet, il faut avoir installé une version de Go supérieure à 1.18. Ensuite, il suffit d'exécuter le script `run.sh` :
```bash
./run.sh
```

Pour exécuter le projet avec des paramètres spécifiques, il faut exécuter le script `run.sh` avec les arguments suivants :

```bash
./run.sh <nombre_d_instances> <reinitialiser_anciennes_sauvegardes> <debug_mode>
```
 avec `<nombre_d_instances>` le nombre d'instances de l'application à exécuter, `<reinitialiser_anciennes_sauvegardes>` devant prendre la valeur `1` si l'on souhaite réinitialiser les anciennes sauvegardes (n'importe quelle autre valeur ou rien sinon) et `<debug_mode>` devant prendre la valeur `1` si l'on souhaite activer le mode débogage : sauvegarde manuelle avec un bouton (n'importe quelle autre valeur ou rien sinon).

## Architecture

L'architecture de l'application est divisée en plusieurs couches :
- Couche applicative
- Couche de contrôle

Cette organisation permet de bien diviser et de distinguer les fonctionnalités applicatives des fonctionnalités de contrôle. En effet, ces deux couches constituent 2 programmes Go distincts qui interagissent entre eux via leurs entrées/sorties standards.

Pour lancer l'application en réseau, le script `run.sh` permet de créer un anneau de taille `N` en définissant correctement les paramètres des sites ainsi que la liaison des entrées/sorties.

Pour cela, il s'assure de nommer correctement chaque élément d'un site `i` (attribuer l'id `i` à la fois au contrôleur et à l'application associée) et d'attribuer tous les id allant de 0 à N-1 (N étant le nombre de sites). 

Ce script permet aussi de créer les fichiers temporaires FIFO qui permettent la liaison des entrées/sorties ainsi qu'un *trap* pour correctement fermer tous les processus lancés par le programme en cas de `CTRL+C`.

Enfin ce script permet de supprimer les anciens fichiers de sorties si l'option est activé et d'activer le mode debug sur l'ensemble des sites si l'option est activée.

En résumé, le schéma ci-dessous permet de représenter l'architecture globale du réseau et de notre projet dans le cas d'un anneau de 4 sites :

![Schéma de l'architecture](doc/schema_anneau.png)

### Couche applicative

La couche applicative implémente une exclusion mutuelle pour distinguer les actions d'écriture et de lecture sur son entrée et sa sortie. Elle conserve un réplicat local de la donnée partagée qui est un fichier `.log` conservant l'historique des modifications permettant de reconstruire le fichier global partagé. 

Deux versions existent alors : 
* La version **debug** qui n'essaie jamais de modifier la donnée partagée et qui attend simplement le clic sur le bouton `save` pour demander l'accès à la section critique à son contrôleur.
* La version **classique** qui essaie de modifier la donnée partagée en demandant l'accès à la section critique dès qu'il y a eu une modification et que le délai s'est écoulé.

Quand l'application demande l'accès à la section critique, l'utilisateur peut toujours continuer ses modifications et la partie visuelle n'est pas stockée localement dans un fichier. 

A la réception de l'accès à la section critique, l'application modifie son fichier local de log avec ses modifications en cours et envoie le contenu de la mise à jour à son contrôleur tout en libérant la section critique.

Si l'application reçoit, du contrôleur, une ou plusieurs modifications du contenu de la donnée partagée, les modifications sont appliquées sur la copie locale du fichier partagé à partir de la dernière version à jour. De plus, l'UI qui affiche le texte à l'utilisateur est mise à jour en appliquant directement les modifications de la version reçu sur la version affichée (même si la version affichée ne correspond pas à la version locale sauvegardée, qui elle, correspond à la version sauvegardée localement par tous les sites).

Voici un résumé de l'architecture d'une instance de l'application :
 
![Schéma de la logique de l'application](doc/schema_application.png)

D'autres messages peuvent aussi être envoyés/reçus par l'application :
* Message de réception de l'indication d'une fermeture de l'application : si l'utilisateur ferme une des fenêtres ouvertes, il faut toutes les fermer pour éviter des soucis de communication. Ce message, une fois reçu, entraîne la fermeture de la fenêtre *Fyne*.
* Message de réception de modification du texte local pour l'initialisation : lors de l'initialisation, si des sites avaient des fichiers sauvegardés différents, celui qui a le plus de lignes est retenu et son contenu est envoyé à tous les sites. A la réception de ce message, l'application du site doit modifier son texte affiché ainsi que celui sauvegardé localement pour garantir la synchronisation globale entre les sites.
* Messsage d'envoi du nombre de lignes et du contenu : servent pour les mêmes raisons que le point précédent pour transmettre l'information au contrôleur
* Message d'envoi d'une demande de coupe au contrôleur pour initier la sauvegarde répartie datée


### Couche de contrôle



