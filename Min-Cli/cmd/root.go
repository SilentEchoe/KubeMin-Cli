package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
)

var cfgFile string

// rootCmd 代表没有调用子命令时的基础命令
var rootCmd = &cobra.Command{
	Use:   "kube-mincli",
	Short: "kube-mincli is a CLI tool that simulates' kubectl' ",
	Long:  `kube-mincli is a CLI tool that simulates' kubectl 'and is used to learn kubernetes API calls.`,
}

// 执行 rootCmd 命令并检测错误
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	// 加载运行初始化配置
	cobra.OnInitialize(initConfig)
	// rootCmd，命令行下读取配置文件，持久化的 flag，全局的配置文件
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.kube-mincli.yaml)")
	// local flag，本地化的配置
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// 初始化配置的一些设置
func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile) // viper 设置配置文件
	} else { // 上面没有指定配置文件，下面就读取 home 下的 .kube-mincli.yaml文件
		// 配置文件参数设置
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".kube-mincli")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil { // 读取配置文件
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
