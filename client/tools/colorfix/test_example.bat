@echo off
REM Script d'exemple pour tester ColorFix
REM Usage: test_example.bat

echo ========================================
echo   ColorFix - Test d'Exemple
echo ========================================
echo.

REM Vérifier que l'exécutable existe
if not exist "colorfix.exe" (
    echo [ERREUR] colorfix.exe introuvable!
    echo Compilez d'abord avec: go build -o colorfix.exe main.go
    pause
    exit /b 1
)

echo [1/3] Preparation...
echo.

REM Créer un dossier de test si nécessaire
if not exist "test_input" mkdir test_input
if not exist "test_output" mkdir test_output

echo [2/3] Exemple de commande:
echo.
echo   Reference: ../../../resources/assets/townhall/Build_townhall_LvA.png
echo   Target:     test_input/ (ou un fichier PNG)
echo   Output:     test_output/
echo.

REM Exemple avec un fichier (décommentez si vous avez un fichier de test)
REM colorfix.exe -reference "../../../resources/assets/townhall/Build_townhall_LvA.png" -target "test_input/test.png" -output "test_output/corrected.png" -intensity 0.3

echo [3/3] Pour utiliser:
echo.
echo   1. Placez vos images PNG dans test_input/
echo   2. Décommentez la ligne dans ce script
echo   3. Relancez ce script
echo.
echo   OU utilisez directement:
echo.
echo   colorfix.exe -reference "chemin/vers/reference.png" -target "chemin/vers/target.png" -output "chemin/vers/output.png" -intensity 0.3
echo.

pause

