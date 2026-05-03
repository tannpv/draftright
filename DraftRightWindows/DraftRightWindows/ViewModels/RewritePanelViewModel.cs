using System;
using System.Collections.ObjectModel;
using System.Threading.Tasks;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using DraftRightWindows.Models;
using DraftRightWindows.Services;
using Windows.ApplicationModel.DataTransfer;

namespace DraftRightWindows.ViewModels;

public partial class RewritePanelViewModel : ObservableObject
{
    // ── Observable properties ──

    [ObservableProperty]
    private string _inputText = string.Empty;

    [ObservableProperty]
    private string _outputText = string.Empty;

    [ObservableProperty]
    private Tone? _selectedTone;

    [ObservableProperty]
    private bool _isLoading;

    [ObservableProperty]
    private string _errorMessage = string.Empty;

    [ObservableProperty]
    private string _usageInfo = string.Empty;

    /// <summary>
    /// Set when the latest /rewrite response was a grammar check
    /// (tone == grammar_check and the response has a `grammar` payload).
    /// RewritePanel observes this and swaps in GrammarCheckView for richer UI.
    /// </summary>
    [ObservableProperty]
    private GrammarResult? _grammarResult;

    // ── Collections ──

    public ObservableCollection<Tone> Tones { get; } = new(
        (Tone[])Enum.GetValues(typeof(Tone))
    );

    // ── Events ──

    /// <summary>Raised when the panel should close.</summary>
    public event EventHandler? CloseRequested;

    /// <summary>
    /// Raised when the user accepts the rewrite and wants to paste it back
    /// into the source application. The string argument is the rewritten text.
    /// </summary>
    public event EventHandler<string>? PasteRequested;

    // ── Commands ──

    [RelayCommand]
    private async Task RewriteAsync(Tone tone)
    {
        if (string.IsNullOrWhiteSpace(InputText))
        {
            ErrorMessage = "No text selected.";
            return;
        }

        SelectedTone = tone;
        ErrorMessage = string.Empty;
        OutputText = string.Empty;
        GrammarResult = null;  // clear any previous grammar UI state
        IsLoading = true;

        try
        {
            var targetLanguage = tone == Tone.Translate
                ? App.Settings.TranslateLanguage
                : null;

            var result = await App.Api.RewriteAsync(
                InputText,
                tone.ApiValue(),
                targetLanguage);

            // Grammar Check returns a structured { grammar: { score, issues } }.
            // Show the structured view for it; for everything else, show plain
            // rewritten_text in the existing TextBlock.
            if (result.Grammar != null && tone == Tone.GrammarCheck)
            {
                GrammarResult = result.Grammar;
                OutputText = string.Empty;
            }
            else
            {
                GrammarResult = null;
                OutputText = result.RewrittenText;
            }
            UsageInfo = $"{result.UsageToday} / {result.DailyLimit} rewrites today";
        }
        catch (ApiException ex)
        {
            ErrorMessage = ex.Message;
        }
        catch (Exception ex)
        {
            ErrorMessage = $"Rewrite failed: {ex.Message}";
        }
        finally
        {
            IsLoading = false;
        }
    }

    [RelayCommand]
    private void Replace()
    {
        if (string.IsNullOrWhiteSpace(OutputText))
            return;

        // Raise PasteRequested — App will handle TextInjector.InjectTextAsync
        // and then close the panel via CloseRequested.
        PasteRequested?.Invoke(this, OutputText);
    }

    [RelayCommand]
    private void Copy()
    {
        if (string.IsNullOrWhiteSpace(OutputText))
            return;

        var dataPackage = new DataPackage();
        dataPackage.SetText(OutputText);
        Clipboard.SetContent(dataPackage);
    }

    [RelayCommand]
    private void Close()
    {
        CloseRequested?.Invoke(this, EventArgs.Empty);
    }
}
