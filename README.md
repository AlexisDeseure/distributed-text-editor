# *Project SR05 Groupe 12* - Application répartie pour un système de traitement textuel décentralisé

Projet réalisé par : 
* Alexis Deseure--Charron
* Antoine Lequeux
* Jessica Devulder
* Timo Allais

## Description du projet

Ce projet a pour but de créer une application distribuée permettant de traiter un fichier texte de manière décentralisée. Cette application comporte des sites reliés en réseau (anneau) entre eux afin de se partager les modifications du fichier texte. L'implémentation de l'algorthme de la file d'attente répartie permet de gérer la concordance entre les différents sites.

## Exécution

Pour exécuter le projet il faut avoir installé une version de Go supérieure à 1.18. Ensuite, il suffit d'exécuter le script `run.sh` en spécifiant le nombre d'instances souhaité et s'il est nécessaire de réinitialiser les anciennes sauvegardes des instances de l'application :

```bash
./run.sh <nombre_d_instances> <reinitialiser_anciennes_sauvegardes>
```
 avec `<nombre_d_instances>` le nombre d'instances de l'application à exécuter et `<reinitialiser_anciennes_sauvegardes>` devant prendre la valeur `1` si l'on souhaite réinitialiser les anciennes sauvegardes (n'importe quel autre valeur ou rien sinon).

## Architecture

L'architecture de l'application est divisée en plusieurs couches :
- Couche applicative
- Couche de contrôle
- Couche d'initialisation

Schéma de l'architecture :

![Schéma de l'architecture](doc/schema_anneau.png)

### Couche applicative


### Couche de contrôle

### Couche d'initialisation
