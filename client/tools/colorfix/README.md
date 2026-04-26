# ColorFix - Outil de Correction de Couleurs

Outil pour harmoniser les couleurs d'assets de bâtiments en utilisant une image de référence.

## Fonctionnalités

- **Transfert de palette** : Applique la palette de couleurs d'une image de référence aux images cibles
- **Préservation d'intensité** : Option pour préserver partiellement l'intensité originale
- **Traitement par lot** : Traite un dossier entier d'images en une fois
- **Transparence préservée** : Maintient les pixels transparents intacts

## Utilisation

### Cas 1 : Corriger une seule image

```bash
colorfix.exe -reference "path/to/reference.png" -target "path/to/target.png" -output "path/to/output.png"
```

### Cas 2 : Corriger toutes les images d'un dossier

```bash
colorfix.exe -reference "path/to/reference.png" -target "path/to/folder/" -output "path/to/output_folder/"
```

Si vous ne spécifiez pas `-output`, les images corrigées seront sauvegardées dans un dossier `_corrected` à côté du dossier source.

### Options

- `-reference` : **Requis**. Image PNG de référence (style souhaité)
- `-target` : **Requis**. Image PNG ou dossier d'images à corriger
- `-output` : Chemin de sortie (fichier ou dossier). Si omis, génère automatiquement un nom
- `-intensity` : Poids de préservation d'intensité (0.0 à 1.0)
  - `0.0` = Transfert complet de couleur (par défaut)
  - `0.3` = Préservation partielle de l'intensité originale (recommandé)
  - `1.0` = Préservation totale de l'intensité (couleurs peu modifiées)

## Exemples

### Exemple 1 : Harmoniser un bâtiment avec le style existant

```bash
# Utiliser le Townhall comme référence
colorfix.exe -reference "resources/assets/townhall/Build_townhall_LvA.png" -target "resources/assets/Tavern/new_tavern.png" -output "resources/assets/Tavern/build_tavern_LvA.png"
```

### Exemple 2 : Traiter tous les bâtiments d'un pack

```bash
# Corriger tous les PNG dans le dossier Tavern
colorfix.exe -reference "resources/assets/townhall/Build_townhall_LvA.png" -target "resources/assets/Tavern/" -intensity 0.3
```

### Exemple 3 : Transfert complet (sans préservation d'intensité)

```bash
colorfix.exe -reference "reference.png" -target "target.png" -intensity 0.0
```

## Comment ça marche ?

L'outil utilise la technique de **Histogram Matching** :

1. **Analyse** : Calcule les histogrammes de couleurs (R, G, B) de l'image de référence
2. **Mapping** : Crée une correspondance entre les valeurs de couleurs de l'image cible et celles de la référence
3. **Application** : Remplace chaque pixel de l'image cible par sa correspondance dans la palette de référence
4. **Préservation** (optionnelle) : Mélange l'intensité originale avec la nouvelle couleur pour un résultat plus naturel

## Conseils

- **Choisir une bonne référence** : Utilisez une image qui représente bien le style visuel souhaité
- **Tester l'intensité** : Commencez avec `-intensity 0.3`, puis ajustez selon le résultat
- **Backup** : L'outil ne modifie pas les fichiers originaux, mais faites des backups quand même !
- **Format** : Fonctionne uniquement avec des PNG (transparence préservée)

## Limitations

- Fonctionne uniquement avec des PNG
- Les images doivent avoir la même structure (même si les couleurs diffèrent)
- Les très grandes images peuvent prendre du temps à traiter

