# 🎨 Démonstration Visuelle - Comment ColorFix Fonctionne

## 📊 Schéma du Processus

```
┌─────────────────────────────────────────────────────────────────┐
│                    PROCESSUS COLORFIX                           │
└─────────────────────────────────────────────────────────────────┘

ÉTAPE 1: ANALYSE
┌──────────────┐
│  RÉFÉRENCE   │  →  Analyse les couleurs (R, G, B)
│  (Townhall)  │  →  Crée une "palette de référence"
│  Beige doux  │
└──────────────┘
        │
        │ "Je veux ces couleurs"
        ▼
┌──────────────┐
│   CIBLE      │  →  Analyse les couleurs actuelles
│  (Tavern)    │  →  Compare avec la référence
│  Bleu vif    │
└──────────────┘
        │
        │ "Je vais harmoniser"
        ▼
┌──────────────┐
│   RÉSULTAT   │  →  Applique la palette de référence
│  (Corrigé)   │  →  Préserve la structure
│  Beige doux  │  →  Garde la transparence
└──────────────┘
```

---

## 🎯 Exemple Concret avec Vraies Couleurs

### AVANT (Pack Original)
```
┌─────────────────────┐
│   🏰 TAVERN         │
│                     │
│   Couleurs:        │
│   • Bleu vif       │
│   • Rouge éclatant │
│   • Vert néon      │
│                     │
│   Style: Pack      │
└─────────────────────┘
```

### RÉFÉRENCE (Style du Jeu)
```
┌─────────────────────┐
│   🏛️ TOWNHALL       │
│                     │
│   Couleurs:        │
│   • Beige doux     │
│   • Marron chaud   │
│   • Crème          │
│                     │
│   Style: Jeu       │
└─────────────────────┘
```

### APRÈS (Corrigé)
```
┌─────────────────────┐
│   🏰 TAVERN         │
│                     │
│   Couleurs:        │
│   • Beige doux     │  ← Harmonisé !
│   • Marron chaud   │  ← Harmonisé !
│   • Crème          │  ← Harmonisé !
│                     │
│   Style: Jeu       │
└─────────────────────┘
```

**Note** : La structure (forme, détails) reste identique, seules les couleurs changent !

---

## 🔄 Transformation Pixel par Pixel

```
PIXEL ORIGINAL          →    PIXEL CORRIGÉ
┌─────────┐                  ┌─────────┐
│ R: 50   │                  │ R: 180  │  ← Mappé depuis la référence
│ G: 100  │   ColorFix  →    │ G: 160  │  ← Mappé depuis la référence
│ B: 200  │                  │ B: 140  │  ← Mappé depuis la référence
│ A: 255  │                  │ A: 255  │  ← Transparence préservée
└─────────┘                  └─────────┘
(Bleu vif)                   (Beige doux)
```

---

## 📈 Histogramme de Couleurs

### Avant Correction
```
Rouge:  ████░░░░░░░░░░░░░░░░░░░░░░  (Peu de rouge)
Vert:   ████████░░░░░░░░░░░░░░░░░░  (Moyen)
Bleu:   ████████████████░░░░░░░░░░  (Beaucoup de bleu)
```

### Référence (Cible)
```
Rouge:  ████████████░░░░░░░░░░░░░░  (Beaucoup de rouge/beige)
Vert:   ████████████░░░░░░░░░░░░░░  (Beaucoup de vert/beige)
Bleu:   ████████░░░░░░░░░░░░░░░░░░  (Moyen)
```

### Après Correction
```
Rouge:  ████████████░░░░░░░░░░░░░░  ← Match la référence !
Vert:   ████████████░░░░░░░░░░░░░░  ← Match la référence !
Bleu:   ████████░░░░░░░░░░░░░░░░░░  ← Match la référence !
```

---

## 🎛️ Effet du Paramètre `-intensity`

### `-intensity 0.0` (Transfert Complet)
```
Original:  ████████░░░░░░░░░░░░░░░░
Résultat:  ░░░░░░░░████████████████  ← 100% nouvelle couleur
```

### `-intensity 0.3` (Recommandé)
```
Original:  ████████░░░░░░░░░░░░░░░░
Résultat:  ████░░░░████████░░░░░░░░  ← 30% original + 70% nouvelle
```

### `-intensity 0.7` (Préservation)
```
Original:  ████████░░░░░░░░░░░░░░░░
Résultat:  ████████████░░░░░░░░░░░░  ← 70% original + 30% nouvelle
```

---

## 💡 Cas d'Usage Réels

### Cas 1 : Pack avec Couleurs Vives
```
Problème:  Pack avec couleurs néon (bleu, vert, rouge vifs)
Solution:  Référence = Townhall (beige doux)
Résultat:  Couleurs harmonisées au style du jeu
```

### Cas 2 : Pack avec Couleurs Sombres
```
Problème:  Pack avec couleurs sombres (gris, noir)
Solution:  Référence = Townhall (beige doux)
Résultat:  Couleurs éclaircies et harmonisées
```

### Cas 3 : Pack avec Couleurs Déjà Proches
```
Problème:  Pack presque bon mais légèrement différent
Solution:  Référence = Townhall + intensity 0.7
Résultat:  Ajustement subtil, pas de changement drastique
```

---

## ✅ Checklist Avant d'Utiliser

- [ ] J'ai une image de référence avec le bon style
- [ ] Mes images à corriger sont en PNG
- [ ] J'ai testé d'abord sur une seule image
- [ ] J'ai sauvegardé mes originaux (l'outil ne les modifie pas)
- [ ] Je sais quelle valeur d'intensité utiliser (commencez par 0.3)

---

## 🚀 Prêt à Essayer ?

1. Placez une image de test dans `Tavern/`
2. Lancez la commande avec le Townhall comme référence
3. Comparez le résultat
4. Ajustez `-intensity` si nécessaire
5. Traitez tout le dossier une fois satisfait !

**C'est aussi simple que ça !** 🎉

