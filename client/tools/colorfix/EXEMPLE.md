# Guide Pratique - ColorFix

## 🎯 Scénario : Vous avez un pack d'assets avec des couleurs différentes

### Situation
- Vous avez des bâtiments dans `resources/assets/Tavern/` avec des couleurs qui ne matchent pas le style du jeu
- Vous voulez harmoniser avec le style existant (ex: Townhall)

---

## 📝 Étape par Étape

### Étape 1 : Préparer les fichiers

1. **Image de référence** : Choisissez un bâtiment qui a le bon style
   - Exemple : `resources/assets/townhall/Build_townhall_LvA.png`

2. **Images à corriger** : Placez-les dans un dossier
   - Exemple : `resources/assets/Tavern/new_tavern.png`

### Étape 2 : Lancer la correction

```bash
# Depuis le dossier racine du projet
cd client/cmd/tools/colorfix

# Commande de base
colorfix.exe -reference "../../../resources/assets/townhall/Build_townhall_LvA.png" -target "../../../resources/assets/Tavern/new_tavern.png" -output "../../../resources/assets/Tavern/build_tavern_LvA.png" -intensity 0.3
```

### Étape 3 : Vérifier le résultat

L'outil va créer une nouvelle image avec les couleurs harmonisées. Comparez :
- **Avant** : `new_tavern.png` (couleurs originales du pack)
- **Après** : `build_tavern_LvA.png` (couleurs harmonisées)

---

## 🔄 Traitement par Lot (Plusieurs Images)

Si vous avez plusieurs images dans le dossier `Tavern/` :

```bash
colorfix.exe -reference "../../../resources/assets/townhall/Build_townhall_LvA.png" -target "../../../resources/assets/Tavern/" -intensity 0.3
```

Cela va :
1. Trouver tous les `.png` dans `Tavern/`
2. Créer un dossier `Tavern_corrected/`
3. Y placer toutes les images corrigées

---

## 🎨 Ajuster l'Intensité

L'option `-intensity` contrôle à quel point les couleurs sont modifiées :

```bash
# Transfert complet (couleurs exactes de la référence)
colorfix.exe -reference "ref.png" -target "target.png" -intensity 0.0

# Mélange naturel (recommandé)
colorfix.exe -reference "ref.png" -target "target.png" -intensity 0.3

# Préservation maximale (couleurs peu modifiées)
colorfix.exe -reference "ref.png" -target "target.png" -intensity 0.7
```

**Conseil** : Commencez avec `0.3`, puis ajustez selon le résultat.

---

## 📊 Exemple Visuel du Processus

```
AVANT (Pack Original)          RÉFÉRENCE (Style du Jeu)      APRÈS (Corrigé)
┌─────────────┐                ┌─────────────┐              ┌─────────────┐
│  🏰 Bleu    │   +  ColorFix  │  🏰 Beige   │    =         │  🏰 Beige   │
│  vif        │   ───────────> │  doux       │              │  harmonisé  │
│  (pack)     │                │  (jeu)      │              │  (résultat) │
└─────────────┘                └─────────────┘              └─────────────┘
```

---

## 🛠️ Dépannage

### Erreur : "Failed to open reference file"
→ Vérifiez que le chemin est correct (utilisez des chemins relatifs ou absolus)

### Erreur : "No PNG files found"
→ Vérifiez que vos fichiers sont bien en `.png` (pas `.jpg` ou autres)

### Résultat trop différent
→ Augmentez `-intensity` (ex: `0.5` ou `0.7`) pour préserver plus les couleurs originales

### Résultat pas assez harmonisé
→ Diminuez `-intensity` (ex: `0.1` ou `0.0`) pour un transfert plus fort

---

## 💡 Astuces

1. **Testez d'abord sur une seule image** avant de traiter tout un dossier
2. **Gardez les originaux** : l'outil ne modifie jamais les fichiers sources
3. **Essayez plusieurs références** : peut-être qu'un autre bâtiment a un meilleur style
4. **Ajustez l'intensité** : chaque pack est différent, testez plusieurs valeurs

---

## 🚀 Commande Rapide (Copier-Coller)

Pour corriger un dossier entier avec le Townhall comme référence :

```bash
cd client/cmd/tools/colorfix
colorfix.exe -reference "../../../resources/assets/townhall/Build_townhall_LvA.png" -target "../../../resources/assets/Tavern/" -intensity 0.3
```

