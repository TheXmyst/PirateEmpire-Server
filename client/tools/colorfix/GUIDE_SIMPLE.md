# 🎨 Guide Simple - ColorFix

## Le Problème
Vous avez des assets avec des **couleurs différentes** mais la **même structure**. Vous voulez les harmoniser.

## La Solution en 3 Étapes

### 📸 Étape 1 : Choisir une Image de Référence
Prenez un bâtiment qui a **déjà le bon style** (ex: Townhall)

```
✅ BON (Référence)          ❌ MAUVAIS (À corriger)
┌─────────────┐              ┌─────────────┐
│  🏰 Beige   │              │  🏰 Bleu    │
│  doux       │              │  vif        │
│  harmonieux │              │  (pack)     │
└─────────────┘              └─────────────┘
```

### 🔧 Étape 2 : Lancer la Commande

**Depuis le dossier `client/cmd/tools/colorfix/`** :

```bash
colorfix.exe -reference "../../../resources/assets/townhall/Build_townhall_LvA.png" -target "../../../resources/assets/Tavern/votre_image.png" -output "../../../resources/assets/Tavern/image_corrigee.png" -intensity 0.3
```

### ✅ Étape 3 : Vérifier le Résultat

Comparez l'image avant/après. Elle devrait maintenant avoir les mêmes couleurs que la référence !

---

## 🎯 Exemple Concret

### Situation
- Vous avez une image `tavern_new.png` dans `Tavern/` avec des couleurs bleues
- Vous voulez qu'elle ait le style beige du Townhall

### Commande

```bash
# Depuis: C:\Users\TheXmyst\Sea-Dogs\client\cmd\tools\colorfix\
colorfix.exe -reference "../../../resources/assets/townhall/Build_townhall_LvA.png" -target "../../../resources/assets/Tavern/tavern_new.png" -output "../../../resources/assets/Tavern/tavern_corrected.png" -intensity 0.3
```

### Résultat
- ✅ `tavern_new.png` reste intact (original)
- ✅ `tavern_corrected.png` est créé avec les couleurs harmonisées

---

## 📁 Traiter Tout un Dossier

Si vous avez plusieurs images dans `Tavern/` :

```bash
colorfix.exe -reference "../../../resources/assets/townhall/Build_townhall_LvA.png" -target "../../../resources/assets/Tavern/" -intensity 0.3
```

Cela crée automatiquement un dossier `Tavern_corrected/` avec toutes les images corrigées.

---

## ⚙️ Paramètre `-intensity`

| Valeur | Effet | Quand l'utiliser |
|--------|-------|------------------|
| `0.0` | Transfert complet | Couleurs très différentes |
| `0.3` | **Recommandé** | La plupart des cas |
| `0.5` | Préservation modérée | Couleurs déjà proches |
| `0.7` | Préservation forte | Juste un petit ajustement |

**Conseil** : Commencez avec `0.3`, puis ajustez selon le résultat.

---

## ❓ Questions Fréquentes

### Q: Ça modifie mes fichiers originaux ?
**R:** Non ! L'outil crée de nouveaux fichiers. Vos originaux restent intacts.

### Q: Ça marche avec quels formats ?
**R:** Uniquement PNG (pour préserver la transparence).

### Q: Combien de temps ça prend ?
**R:** Quelques secondes par image, même pour des images grandes.

### Q: Je peux utiliser n'importe quelle image comme référence ?
**R:** Oui, mais utilisez une image qui représente bien le style souhaité.

---

## 🚀 Commande Rapide (Copier-Coller)

```bash
cd client/cmd/tools/colorfix
colorfix.exe -reference "../../../resources/assets/townhall/Build_townhall_LvA.png" -target "../../../resources/assets/Tavern/" -intensity 0.3
```

C'est tout ! 🎉

